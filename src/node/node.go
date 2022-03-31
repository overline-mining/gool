package main

import (
	"context"
	"flag"
	"time"

	"github.com/wavesplatform/gowaves/pkg/libs/ntptime"
	"github.com/wavesplatform/gowaves/pkg/types"
	"github.com/wavesplatform/gowaves/pkg/util/fdlimit"

	"github.com/overline-mining/gool/src/common"

	"go.uber.org/zap"

	"math/rand"

	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	dht "github.com/anacrolix/dht/v2"
	"github.com/anacrolix/dht/v2/krpc"
	"github.com/anacrolix/log"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/tracker"
	trHttp "github.com/anacrolix/torrent/tracker/http"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	chain "github.com/overline-mining/gool/src/blockchain"
	"github.com/overline-mining/gool/src/genesis"
	"github.com/overline-mining/gool/src/protocol/messages"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	//"github.com/overline-mining/gool/src/transactions"
	"github.com/autom8ter/dagger"
	db "github.com/overline-mining/gool/src/database"
	"github.com/overline-mining/gool/src/validation"
	lz4 "github.com/pierrec/lz4/v4"
	probar "github.com/schollz/progressbar/v3"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

var (
	logLevel            = flag.String("log-level", "INFO", "Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	blockchainType      = flag.String("blockchain-type", "mainnet", "Blockchain type: mainnet/testnet/integration")
	limitAllConnections = flag.Uint("limit-connections", 60, "Total limit of network connections, both inbound and outbound. Divided in half to limit each direction. Default value is 60.")
	dbFileDescriptors   = flag.Int("db-file-descriptors", 500, "Maximum allowed file descriptors count that will be used by state database. Default value is 500.")
	olTestPeer          = flag.String("test-peer", "", "Specify an exact peer (ip:port) to connect to instead of using tracker")
	olWorkDir           = flag.String("ol-workdir", ".overline", "Specify the working directory for the node")
	dbFileName          = flag.String("db-file-name", "overline.boltdb", "The file we're going to write the blockchain to within olWorkDir")
	validateFullChain   = flag.Bool("full-validation", false, "Run a slow but complete validation of your local blockchain DB")
)

type ConcurrentPeerMap struct {
	PeerList map[string]trHttp.Peer
	Lock     sync.Mutex
}

func (m *ConcurrentPeerMap) AddPeer(name string, peer trHttp.Peer) {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	m.PeerList[name] = peer
}

func (m *ConcurrentPeerMap) Get(name string) trHttp.Peer {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	return m.PeerList[name]
}

type OverlinePeer struct {
	Conn   net.Conn
	Height uint64
	ID     []byte
}

type OverlineMessage struct {
	ID     uint64
	Type   string
	PeerID []byte
	Value  []byte
}

type OverlineMessageHandler struct {
	Mu       *sync.Mutex // mutex is for Messages, shared across threads
	Peer     OverlinePeer
	Messages *[]OverlineMessage
	ID       []byte
	buf      [0x100000]byte // 1 MB buffer for work
	buflen   int
}

func (mh *OverlineMessageHandler) Initialize(conn net.Conn) {
	mh.Mu.Lock()
	defer mh.Mu.Unlock()

	mh.buflen = 0

	peer_id, nbuf, err := handshake_peer(conn, mh.ID, &mh.buf)
	mh.buflen = nbuf
	if err != nil {
		zap.S().Debugf("Failed to connect to %v: %v", hex.EncodeToString(peer_id), err)
		conn.Close()
		return
	}
	zap.S().Debugf("Completed handshake: %v -> %v", hex.EncodeToString(mh.ID), hex.EncodeToString(peer_id))
	mh.Peer = OverlinePeer{Conn: conn, ID: peer_id, Height: 0}
}

type IBDWorkList struct {
	Mu            sync.Mutex
	AllowedBlocks map[uint64]uint64
}

func (mh *OverlineMessageHandler) Run() {
	messageCounter := uint64(0)
	for {
		n := 0
		cursor := 0
		currentMessage := make([]byte, 0)
		var err error
		if mh.buflen < 4 {
			n, err = mh.Peer.Conn.Read(mh.buf[mh.buflen:])
			if err != nil {
				zap.S().Errorf("closing peer %v: %v", hex.EncodeToString(mh.Peer.ID), err)
				return
			}
			mh.buflen += n
		} else {
			n = mh.buflen
		}
		msgLen := int(binary.BigEndian.Uint32(mh.buf[cursor : cursor+4]))
		cursor += 4
		if (n - cursor) < msgLen {
			// get the block part out of the buffer
			currentMessage = append(currentMessage, mh.buf[cursor:n]...)
			ntot := n - cursor
			cursor += n - cursor
			// retreive the rest of the message
			for len(currentMessage) < msgLen {
				n, err := mh.Peer.Conn.Read(mh.buf[0:])
				if err != nil {
					zap.S().Errorf("closing peer %v: %v", hex.EncodeToString(mh.Peer.ID), err)
					return
				}
				mh.buflen = n
				cursor = 0
				if n+ntot < msgLen {
					currentMessage = append(currentMessage, mh.buf[:n]...)
					ntot += n
				} else {

					currentMessage = append(currentMessage, mh.buf[:(msgLen-ntot)]...)
					backup := 0
					for currentMessage[len(currentMessage)-backup-1] == 0 || currentMessage[len(currentMessage)-backup-2] == 0 {
						backup++
					}
					currentMessage = currentMessage[:len(currentMessage)-backup]
					cursor = msgLen - ntot - backup
					/*
						begin := cursor - 20
						if cursor-20 < 0 {
							begin = 0
						}
					*/
					ntot += (msgLen - ntot)
					break
				}
			}
		} else {
			zap.S().Debugf("OverlineMessageHandler::Run -> had the meesage locally!")
			currentMessage = append(currentMessage, mh.buf[cursor:cursor+msgLen]...)
			cursor += msgLen
		}
		// advance buffer to the start of the next message
		copy(mh.buf[0:], mh.buf[cursor:])
		mh.buflen = mh.buflen - cursor
		cursor = 0

		// put the message into the queue
		parts := bytes.Split(currentMessage, []byte(messages.SEPARATOR))
		if len(parts[0]) == 0 {
			err := errors.New("OverlineMessageHandler::Run -> Invalid message, length of messages parts is zero!")
			checkError(err)
		}
		if len(parts[0]) != 7 {
			err := errors.New(fmt.Sprintf("OverlineMessageHandler::Run -> Invalid message, does not begin with a valid message type specifier: %v", string(parts[0])))
			checkError(err)
		}
		msgType := string(parts[0])

		mh.Mu.Lock()
		newMessage := OverlineMessage{ID: messageCounter, Type: msgType, PeerID: mh.Peer.ID, Value: bytes.Join(parts[1:], []byte(messages.SEPARATOR))}
		messageCounter++
		*mh.Messages = append(*mh.Messages, newMessage)
		zap.L().Debug("OverlineMessageHandler::Run -> Appended message to queue: ",
			zap.Uint64("ID", newMessage.ID),
			zap.String("Type", newMessage.Type),
			zap.String("PeerID", hex.EncodeToString(newMessage.PeerID)),
			zap.Int("msgLen", len(newMessage.Value)))
		mh.Mu.Unlock()
	}
}

func ExtractRanges(a []uint64) ([][2]uint64, error) {
	if len(a) == 0 {
		return make([][2]uint64, 0), nil
	}
	var parts [][2]uint64
	for n1 := 0; ; {
		n2 := n1 + 1
		for n2 < len(a) && a[n2] == a[n2-1]+1 {
			n2++
		}
		therange := [2]uint64{a[n1], 0}
		if n2 >= n1+2 {
			therange[1] = a[n2-1]
		}
		parts = append(parts, therange)
		if n2 == len(a) {
			break
		}
		if a[n2] == a[n2-1] {
			return make([][2]uint64, 0), errors.New(fmt.Sprintf(
				"sequence repeats value %d", a[n2]))
		}
		if a[n2] < a[n2-1] {
			return make([][2]uint64, 0), errors.New(fmt.Sprintf(
				"sequence not ordered: %d < %d", a[n2], a[n2-1]))
		}
		n1 = n2
	}
	return parts, nil
}

func PrintProgramHeader() {
	zap.S().Info("  .-.      ")
	zap.S().Info(" (o o) boo!")
	zap.S().Info(" | O \\     ")
	zap.S().Info("  \\   \\    ")
	zap.S().Info("   `~~~')  ")
	zap.S().Infof("GoOL (/ɡuːl/) version: %s", common.GetVersion())
}

func main() {
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	maxFDs, err := fdlimit.MaxFDs()
	if err != nil {
		zap.S().Fatalf("Initialization failure: %v", err)
	}
	_, err = fdlimit.RaiseMaxFDs(maxFDs)
	if err != nil {
		zap.S().Fatalf("Initialization failure: %v", err)
	}
	if maxAvailableFileDescriptors := int(maxFDs) - int(*limitAllConnections) - 10; *dbFileDescriptors > maxAvailableFileDescriptors {
		zap.S().Fatalf("Invalid 'db-file-descriptors' flag value (%d). Value shall be less or equal to %d.", *dbFileDescriptors, maxAvailableFileDescriptors)
	}

	common.SetupLogger(*logLevel)

	PrintProgramHeader()

	//ctx, cancel := context.WithCancel(context.Background())

	/*
		ntpTime, err := getNtp(ctx)
		if err != nil {
			zap.S().Error(err)
			cancel()
			return
		}

		zap.S().Info(ntpTime)
	*/

	// make the workdir if it does not exist
	err = os.MkdirAll(*olWorkDir, 0700)
	checkError(err)

	// database testing
	startingHeight := uint64(0)
	dbFilePath := filepath.Join(*olWorkDir, *dbFileName)
	gooldb := db.OverlineDB{Config: db.DefaultOverlineDBConfig()}
	err = gooldb.Open(dbFilePath)

	if err != nil {
		startingHeight = 1
		zap.S().Warn("Blockchain file was uninitialized - creating genesis block")
		var gblock *p2p_pb.BcBlock
		gblock, err = genesis.BuildGenesisBlock(*olWorkDir)
		if err != nil {
			zap.S().Fatal(err)
			os.Exit(1)
		}
		err = gooldb.AddBlock(gblock)
	} else {
		startingHeight = gooldb.SerializedHeight()
	}
	defer gooldb.Close()

	checkError(err)

	// add ctrl-c catcher for closing the database
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		gooldb.Close()
		os.Exit(1)
	}()

	// run full local validation if requested
	if *validateFullChain {
		gooldb.FullLocalValidation()
	}

	// start the gool ingestion thread
	gooldb.Run()

	ibdWorkList := IBDWorkList{AllowedBlocks: make(map[uint64]uint64)}

	// create the chain ingester
	ovlChain := chain.OverlineBlockchain{
		BlockGraph: dagger.NewGraph(),
		DB:         &gooldb,
		Heads:      make(map[string]bool),
	}

	id_bytes := make([]byte, 32)
	rand.Read(id_bytes)
	id := metainfo.HashBytes(id_bytes)
	zap.S().Infof("ID     : %v", hex.EncodeToString(id_bytes))
	zap.S().Infof("ID Hash: %v", id)
	// bcnode mainnet infohash
	infoHash_hex := "716ca5c568509b3652a21dd076e1bf842583267c"
	infoHash := metainfo.NewHashFromHex(infoHash_hex)

	trackers := []string{
		"udp://tds-r3.blockcollider.org:16060/announce",
		"udp://139.180.221.213:16060/announce",
		"udp://52.208.110.140:16060/announce",
		"udp://internal.xeroxparc.org:16060/announce",
		"udp://reboot.alcor1436.com:16060/announce",
		"udp://sa1.alcor1436.com:16060/announce",
	}

	zap.S().Infof("Infohash -> %v", infoHash)
	zap.S().Debugf("Checking the following trackers for peers: %v", trackers)

	allPeers := ConcurrentPeerMap{PeerList: make(map[string]trHttp.Peer)}
	if len(*olTestPeer) == 0 {
		for _, tr := range trackers {
			go func(tr string) {
				for {
					resp, err := tracker.Announce{
						TrackerUrl: tr,
						Request: tracker.AnnounceRequest{
							InfoHash: infoHash,
							PeerId:   id,
							Port:     16060,
						},
					}.Do()
					if err == nil {
						zap.S().Debugf("%v has %v peers!", tr, len(resp.Peers))
						for _, peer := range resp.Peers {
							allPeers.AddPeer(fmt.Sprintf("%v", peer), peer)
						}
					} else {
						zap.S().Error(err)
					}
					time.Sleep(time.Minute * 1)
				}
			}(tr)
		}
		// wait for announce to fill the peer list the first time
		peerListLen := uint64(0)
		peerListLenLast := uint64(0)
		for {
			peerListLen = uint64(len(allPeers.PeerList))
			if peerListLen == 0 {
				time.Sleep(time.Millisecond * 250)
				continue
			}
			if peerListLen == peerListLenLast {
				break
			}
			peerListLenLast = peerListLen
		}
	} else {
		for _, peer := range strings.Split(*olTestPeer, ",") {
			split := strings.Split(peer, ":")
			port, err := strconv.ParseUint(split[1], 10, 16)
			checkError(err)
			allPeers.PeerList[*olTestPeer] = trHttp.Peer{IP: net.ParseIP(split[0]), Port: int(port)}
		}
	}

	unifiedPeerList := []string{}
	for name, peer := range allPeers.PeerList {
		zap.S().Debugf("%v -> peer: %v port: %v id: %v", name, peer.IP, peer.Port, peer.ID)
		unifiedPeerList = append(unifiedPeerList, peer.IP.String()+fmt.Sprintf(":%d", peer.Port+1))
	}

	// build dht

	dht.DefaultGlobalBootstrapHostPorts = unifiedPeerList

	dhtConfig := dht.NewDefaultServerConfig()
	dhtConfig.NodeId = krpc.IdFromString(id.AsString())
	dhtConfig.StartingNodes = func() ([]dht.Addr, error) { return dht.GlobalBootstrapAddrs("udp4") }
	dhtConfig.Conn, err = net.ListenPacket("udp4", "0.0.0.0:16061")
	if err != nil {
		zap.S().Info(err)
		return
	}
	dhtConfig.Logger.FilterLevel(log.Debug)
	dhtConfig.OnQuery = func(query *krpc.Msg, source net.Addr) (propagate bool) {
		zap.S().Infof("Received query (%v): %v", source, query)
		propagate = true
		return
	}

	dhtServer, err := dht.NewServer(dhtConfig)
	if err != nil {
		zap.S().Info(err)
		return
	}

	dhtServer.Announce(infoHash, 0, false)

	olMessageMu := sync.Mutex{}
	olMessages := make([]OverlineMessage, 0)
	olHandlerMapMu := sync.Mutex{}
	olMessageHandlers := make(map[string]OverlineMessageHandler)
	dialer := net.Dialer{Timeout: time.Millisecond * 5000}

	var waitForPeers sync.WaitGroup

	for _, peer := range allPeers.PeerList {
		waitForPeers.Add(1)
		go func() {
			defer waitForPeers.Done()

			peerString := peer.IP.String() + fmt.Sprintf(":%d", peer.Port+1)

			zap.S().Debugf("Working on peer: %v", peerString)

			tcpAddr, err := net.ResolveTCPAddr("tcp4", peerString)
			if err != nil {
				zap.S().Debug(err)
				return
			}

			conn, err := dialer.Dial("tcp4", tcpAddr.String())
			if err != nil {
				zap.S().Debug(err)
				return
			}

			handler := OverlineMessageHandler{Mu: &olMessageMu, Messages: &olMessages, ID: id_bytes}
			handler.Initialize(conn)
			go handler.Run()
			olHandlerMapMu.Lock()
			olMessageHandlers[hex.EncodeToString(handler.Peer.ID)] = handler
			olHandlerMapMu.Unlock()
		}()
		time.Sleep(time.Millisecond * 10)
	}

	waitForPeers.Wait()

	olHandlerMapMu.Lock()
	zap.S().Infof("Successful handshakes with %d nodes!", len(olMessageHandlers))
	olHandlerMapMu.Unlock()

	ibdBar := probar.Default(int64(startingHeight+1), "initial block download ->")
	ibdBar.Add64(int64(startingHeight))

	go func() {
		for {
			olMessageMu.Lock()
			if len(olMessages) > 0 {
				zap.S().Debugf("There are %v messages in the queue!", len(olMessages))
				for len(olMessages) > 0 {
					oneMessage := (olMessages)[0]
					olMessages = (olMessages)[1:]
					zap.S().Debugf("Popped message %v of type %v from %v", oneMessage.ID, oneMessage.Type, hex.EncodeToString(oneMessage.PeerID))
					switch oneMessage.Type {
					case messages.DATA:
						blocks := p2p_pb.BcBlocks{}
						err = proto.Unmarshal(oneMessage.Value, &blocks)
						if gooldb.IsInitialBlockDownload() {
							goodBlocks := p2p_pb.BcBlocks{}
							ibdWorkList.Mu.Lock()
							for _, block := range blocks.Blocks {
								if _, ok := ibdWorkList.AllowedBlocks[block.GetHeight()]; ok {
									goodBlocks.Blocks = append(goodBlocks.Blocks, block)
									delete(ibdWorkList.AllowedBlocks, block.GetHeight())
								}
							}
							ibdWorkList.Mu.Unlock()
							blocks = goodBlocks
							ibdBar.Add(len(blocks.Blocks))
						}
						if err == nil {
							gooldb.AddBlockRange(&blocks)
						} else {
							zap.S().Error(err)
						}
					case messages.BLOCK:
						b := new(p2p_pb.BcBlock)
						err = proto.Unmarshal(oneMessage.Value, b)
						checkError(err)
						isValid, err := validation.IsValidBlock(b)
						if isValid {
							peerIDHex := hex.EncodeToString(oneMessage.PeerID)
							msgHandler := olMessageHandlers[peerIDHex]
							msgHandler.Peer.Height = b.GetHeight()
							olMessageHandlers[peerIDHex] = msgHandler
							zap.S().Debugf("Received Valid BLOCK: Set Height of %v to %v", peerIDHex, b.GetHeight())
							ovlChain.AddBlock(b)
							if !gooldb.IsInitialBlockDownload() {
								gooldb.AddBlock(b)
							} else {
								height_i64 := int64(b.GetHeight())
								if height_i64 > ibdBar.GetMax64() {
									ibdBar.ChangeMax64(height_i64)
								}
							}
						} else {
							zap.S().Warnf("Received Invalid BLOCK: %v -> %v", b.GetHeight(), err)
						}
					case messages.TX:
						tx := new(p2p_pb.Transaction)
						err = proto.Unmarshal(oneMessage.Value, tx)
						checkError(err)
						peerIDHex := hex.EncodeToString(oneMessage.PeerID)
						zap.S().Debugf("Received broadcasted TX %v from %v", tx.GetHash(), peerIDHex)
						if !gooldb.IsInitialBlockDownload() {
							gooldb.AddTransaction(tx)
						}
					default:
						zap.S().Debugf("Throwing away: %v->%v", hex.EncodeToString(oneMessage.PeerID), oneMessage.Type)
					}

				}
				olMessageMu.Unlock()
			} else {
				olMessageMu.Unlock()
				time.Sleep(time.Millisecond * 50)
			}
		}
	}()

	/*
		go func() {
			blockStride := uint64(10)
			iStride := uint64(0)
			topRange := uint64(startingHeight + (iStride+1)*blockStride + 1)
			for {
				if !gooldb.IsInitialBlockDownload() {
					break
				}
				olHandlerMapMu.Lock()
				for peer, messageHandler := range olMessageHandlers {
					olMessageMu.Lock()
					if messageHandler.Peer.Height == 0 || messageHandler.Peer.Height < topRange {
						olMessageMu.Unlock()
						continue
					}
					low, high := uint64(iStride*blockStride), uint64((iStride+1)*blockStride)
					low += startingHeight + 1
					high += startingHeight + 1
					if high > messageHandler.Peer.Height {
						high = messageHandler.Peer.Height
					}
					ibdWorkList.Mu.Lock()
					for i := low; i < high; i++ {
						ibdWorkList.AllowedBlocks[i] = low
					}
					ibdWorkList.Mu.Unlock()
					reqstr := messages.GET_DATA + messages.SEPARATOR + fmt.Sprintf("%d%s%d", low, messages.SEPARATOR, high)
					reqLen := len(reqstr)
					request := make([]byte, reqLen+4)
					binary.BigEndian.PutUint32(request[0:], uint32(reqLen))
					copy(request[4:], []byte(reqstr))

					zap.S().Debugf("Sending request: %v -> %v", peer, reqstr)

					// write the request to the connection
					n, err := messageHandler.Peer.Conn.Write(request)
					//zap.S().Debugf("Wrote %v bytes to the outbound connection!", n)
					if n != len(request) {
						zap.S().Fatal("Fatal error: didn't write complete request to outbound connection!")
						os.Exit(1)
					}
					checkError(err)
					olMessageMu.Unlock()

					iStride += 1
					if iStride%100 == 0 { // if we've submitted a request for 1000 blocks - wait until we have received them all
						zap.S().Debugf("Submitted block requests for range [%v,%v]", startingHeight+(iStride-100)*blockStride, startingHeight+iStride*blockStride)
						for {
							ibdWorkList.Mu.Lock()
							nBlocksRemaining := len(ibdWorkList.AllowedBlocks)
							ibdWorkList.Mu.Unlock()
							if nBlocksRemaining == 0 {
								break
							} else {
								zap.S().Debugf("waiting for %v blocks to arrive...", nBlocksRemaining)
								time.Sleep(time.Millisecond * 500)
							}
						}
					}

					topRange = uint64(startingHeight + (iStride+1)*blockStride + 1)
				}
				olHandlerMapMu.Unlock()
				time.Sleep(time.Millisecond * 250)
			}
			zap.L().Info("Initial block download has completed.")
		}()
	*/

	defer func() {
		olHandlerMapMu.Lock()
		for _, handler := range olMessageHandlers {
			handler.Peer.Conn.Close()
		}
		olHandlerMapMu.Unlock()
	}()

	for {
		time.Sleep(time.Second * 1)
	}
	return
}

func handshake_peer(conn net.Conn, id []byte, buf *[0x100000]byte) ([]byte, int, error) {
	peerHandshake := make([]byte, 0)

	lenSize := binary.PutUvarint(buf[0:], uint64(len(id)))

	copy(buf[lenSize:], id)

	zap.S().Debugf("handshake -> sending lenSize: %v id: %v", lenSize, hex.EncodeToString(buf[lenSize:len(id)]))

	_, err := conn.Write(buf[:lenSize+len(id)])
	if err != nil {
		return make([]byte, 0), -1, err
	}

	// receive the full message and then decode it
	n, err := conn.Read(buf[0:])
	if err != nil {
		return make([]byte, 0), -1, err
	}
	ntot := n
	// there are four possibilities for handshake replies:
	// 1 - the first read is the length of handshake (then we need to read more)
	// 2 - the first read is the length-encoded full handshake reply
	// 3 - the first read is the L-E full handshake reply and the handshake block
	// 4 - the first read   is the L-E full handshake reply and part of the handshake block
	zap.S().Debugf("peer handshake -> received buffer of length %d", ntot)

	// all cases deal with length of peer handshake (always one byte for sane peers)
	cursor := uint64(1)
	lenPeer, _ := binary.Uvarint(buf[:cursor])
	peerHandshake = append(peerHandshake, buf[:cursor]...)

	if lenPeer != 32 && lenPeer != 20 {
		return make([]byte, 0), -1, errors.New(fmt.Sprintf("Invalid peer handshake length: %v", lenPeer))
	}

	// if we only received the first few bytes of the peer handshake
	// ask for more
	nPeerRemain := uint64(0)
	if uint64(ntot) < lenPeer+1 {
		zap.S().Debugf("peer handshake -> received partial peer handshake, requesting more data")
		nPeerRemain = lenPeer + 1 - uint64(n)
		n, err = conn.Read(buf[n:])
		if err != nil {
			return make([]byte, 0), -1, err
		}
		ntot += n
		zap.S().Debugf("peer handshake -> received buffer of length %d, expecting %v", n, nPeerRemain)
		peerHandshake = append(peerHandshake, buf[cursor:cursor+lenPeer]...)
	} else { // process the rest of buf
		peerHandshake = append(peerHandshake, buf[cursor:cursor+lenPeer]...)
	}
	zap.S().Debugf("peer handshake -> full handshake: (%v) %v", lenPeer, hex.EncodeToString(peerHandshake[cursor:cursor+lenPeer]))
	cursor += lenPeer

	copy(buf[0:], buf[cursor:ntot])

	return peerHandshake[1:lenPeer], int(ntot - int(cursor)), nil
}

func SerializeBlocks(db *bolt.DB, blocks p2p_pb.BcBlocks) error {
	c := &lz4.CompressorHC{}
	blocksBytes, err := proto.Marshal(&blocks)
	compressionBuf := make([]byte, len(blocksBytes))
	checkError(err)
	zap.S().Infof("Blocklist: is %v bytes long, consisting of %v blocks", len(blocksBytes), len(blocks.Blocks))
	nCompressed, err := c.CompressBlock(blocksBytes, compressionBuf)
	checkError(err)
	zap.S().Infof("Blocklist: compressed to %v bytes!", nCompressed)

	return db.Batch(func(tx *bolt.Tx) error {
		// block chunks keyed by the first hash in the chunk
		chunks, err := tx.CreateBucketIfNotExists([]byte("OVERLINE-BLOCK-CHUNKS"))
		if err != nil {
			return err
		}
		// block hashes to chunks that store that block
		block2chunk, err := tx.CreateBucketIfNotExists([]byte("OVERLINE-BLOCK-CHUNK-MAP"))
		if err != nil {
			return err
		}
		// block height to block hash
		height2hash, err := tx.CreateBucketIfNotExists([]byte("OVERLINE-BLOCK-HEIGHT-TO-HASH"))
		if err != nil {
			return err
		}
		tx2hash, err := tx.CreateBucketIfNotExists([]byte("OVERLINE-TX-TO-BLOCK"))
		if err != nil {
			return err
		}
		syncInfo, err := tx.CreateBucketIfNotExists([]byte("SYNC-INFO"))
		if err != nil {
			return err
		}

		// first store the compressed chunk of blocks
		// everythin else is referencing information
		chunkHash, _ := hex.DecodeString(blocks.Blocks[0].GetHash())
		err = chunks.Put(chunkHash, compressionBuf[:nCompressed])
		if err != nil {
			return err
		}

		for _, block := range blocks.Blocks {
			blockHash, _ := hex.DecodeString(block.GetHash())
			blockHeight := make([]byte, 8)
			binary.BigEndian.PutUint64(blockHeight, block.GetHeight())

			// eventually we'll need to figure out the winning
			// block and only commit that!
			err = height2hash.Put(blockHeight, blockHash)
			if err != nil {
				return err
			}

			for _, tx := range block.Txs {
				txHash, _ := hex.DecodeString(tx.GetHash())
				err = tx2hash.Put(txHash, blockHash)
				if err != nil {
					return err
				}
			}

			err = block2chunk.Put(blockHash, chunkHash)
			if err != nil {
				return err
			}

			err = syncInfo.Put([]byte("LastWrittenBlockHeight"), blockHeight)
			err = syncInfo.Put([]byte("LastWrittenBlockHash"), blockHash)
			if err != nil {
				return err
			}
		}
		return err
	})
}

func getNtp(ctx context.Context) (types.Time, error) {
	if *blockchainType == "integration" {
		return ntptime.Stub{}, nil
	}
	tm, err := ntptime.TryNew("pool.ntp.org", 10)
	if err != nil {
		return nil, err
	}
	go tm.Run(ctx, 2*time.Minute)
	return tm, nil
}

func checkError(err error) {
	if err != nil {
		zap.S().Fatalf("Fatal error: %s", err.Error())
		os.Exit(1)
	}
}
