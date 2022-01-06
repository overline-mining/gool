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
	"strconv"
	"strings"
	"sync"

	"github.com/overline-mining/gool/src/protocol/messages"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"github.com/overline-mining/gool/src/validation"
	lz4 "github.com/pierrec/lz4/v4"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

var (
	logLevel            = flag.String("log-level", "INFO", "Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	blockchainType      = flag.String("blockchain-type", "mainnet", "Blockchain type: mainnet/testnet/integration")
	limitAllConnections = flag.Uint("limit-connections", 60, "Total limit of network connections, both inbound and outbound. Divided in half to limit each direction. Default value is 60.")
	dbFileDescriptors   = flag.Int("db-file-descriptors", 500, "Maximum allowed file descriptors count that will be used by state database. Default value is 500.")
	olTestPeer          = flag.String("test-peer", "", "Specify an exact peer (ip:port) to connect to instead of using tracker")
	dbFilePath          = flag.String("db-file-path", "overline.boltdb", "The file we're going to write the blockchain to")
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
	Type   string
	PeerID []byte
	Value  []byte
}

type OverlineMessageHandler struct {
	Mu       sync.Mutex // mutex is for Messages, shared across threads
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
		zap.S().Infof("Failed to connect to %v: %v", hex.EncodeToString(peer_id), err)
		conn.Close()
		return
	}
	zap.S().Infof("Completed handshake: %v -> %v", hex.EncodeToString(mh.ID), hex.EncodeToString(peer_id))
	mh.Peer = OverlinePeer{Conn: conn, ID: peer_id, Height: 0}
}

func (mh *OverlineMessageHandler) Run() {
	for {
		n := 0
		cursor := 0
		currentMessage := make([]byte, 0)
		var err error
		zap.S().Debugf("OverlineMessageHandler::Run START")
		zap.S().Debugf("OverlineMessageHandler::Run -> mh.buflen is %v bytes", mh.buflen)
		if mh.buflen < 4 {
			n, err = mh.Peer.Conn.Read(mh.buf[mh.buflen:])
			checkError(err)
			mh.buflen += n
			zap.S().Debugf("OverlineMessageHandler::Run -> read %v bytes", n)
			zap.S().Debugf("OverlineMessageHandler::Run -> cursor is %v", cursor)
		} else {
			n = mh.buflen
		}
		msgLen := int(binary.BigEndian.Uint32(mh.buf[cursor : cursor+4]))
		zap.S().Debugf("OverlineMessageHandler::Run -> expect message of length %v", msgLen)
		cursor += 4
		if (n - cursor) < msgLen {
			// get the block part out of the buffer
			currentMessage = append(currentMessage, mh.buf[cursor:n]...)
			ntot := n - cursor
			cursor += n - cursor
			// retreive the rest of the message
			for len(currentMessage) < msgLen {
				n, err := mh.Peer.Conn.Read(mh.buf[0:])
				checkError(err)
				mh.buflen = n
				cursor = 0
				if n+ntot < msgLen {
					currentMessage = append(currentMessage, mh.buf[:n]...)
					ntot += n
					cursor = n
				} else {
					currentMessage = append(currentMessage, mh.buf[:(msgLen-ntot)]...)
					cursor = msgLen - ntot
					ntot += (msgLen - ntot)
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
		zap.S().Debugf("OverlineMessageHandler::Run -> Received message of type: %v", msgType)

		mh.Mu.Lock()
		newMessage := OverlineMessage{Type: msgType, PeerID: mh.Peer.ID, Value: bytes.Join(parts[1:], []byte(messages.SEPARATOR))}
		*mh.Messages = append(*mh.Messages, newMessage)
		zap.S().Debugf("OverlineMessageHandler::Run -> Appended message to queue: %v %v %v", newMessage.Type, hex.EncodeToString(newMessage.PeerID), len(newMessage.Value))
		mh.Mu.Unlock()
	}
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
	zap.S().Info(messages.HANDSHAKE)

	// database testing
	db, err := bolt.Open(*dbFilePath, 0600, nil)
	checkError(err)
	defer db.Close()

	startingHeight := uint64(0)
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("SYNC-INFO"))
		if b == nil {
			return errors.New("Unitialized blockchain file!")
		}
		heightBytes := b.Get([]byte("LastWrittenBlockHeight"))
		startingHeight = binary.BigEndian.Uint64(heightBytes)
		zap.S().Infof("The LastWrittenBlockHeight is: %d", startingHeight)
		return nil
	})
	if err != nil {
		zap.S().Warn("Blockchain file was uninitialized - will sync from genesis")
		startingHeight = 0
	} else {
		decompressionBuf := make([]byte, 0x3000000) // 50MB should be enough to cover
		if *validateFullChain {
			err = db.View(func(tx *bolt.Tx) error {
				zap.S().Debug("Performing complete validation of chain before connecting.")
				heights := tx.Bucket([]byte("OVERLINE-BLOCK-HEIGHT-TO-HASH"))
				chunks := tx.Bucket([]byte("OVERLINE-BLOCK-CHUNKS"))
				block2chunk := tx.Bucket([]byte("OVERLINE-BLOCK-CHUNK-MAP"))

				c := heights.Cursor()
				seek := uint64(1) //uint64(2171001) //uint64(1074498) //uint64(1074552) //uint64(202669)
				seekBytes := make([]byte, 8)
				binary.BigEndian.PutUint64(seekBytes, seek)

				currentChunk := make([]byte, 32)
				blockMap := make(map[string]*p2p_pb.BcBlock)
				for k, v := c.Seek(seekBytes); k != nil; k, v = c.Next() {
					height := binary.BigEndian.Uint64(k)
					hash := hex.EncodeToString(v)
					chunkHash := block2chunk.Get(v)
					chunk := chunks.Get(chunkHash)

					if bytes.Compare(currentChunk, chunkHash) != 0 {
						blockMap = make(map[string]*p2p_pb.BcBlock) // reset the blockmap
						nDecompressed, err := lz4.UncompressBlock(chunk, decompressionBuf[0:])
						if err != nil {
							return err
						}
						blockList := p2p_pb.BcBlocks{}
						err = proto.Unmarshal(decompressionBuf[:nDecompressed], &blockList)
						if err != nil {
							return err
						}
						for _, block := range blockList.Blocks {
							blockMap[block.GetHash()] = block
						}
						strCurrentChunk := hex.EncodeToString(currentChunk)
						strChunkHash := hex.EncodeToString(chunkHash)
						zap.S().Debugf("Updating chunk hash from %s to %s", common.BriefHash(strCurrentChunk), common.BriefHash(strChunkHash))
						copy(currentChunk, chunkHash)
					}
					block := blockMap[hash]
					isValid, err := validation.IsValidBlock(block)
					if err != nil {
						zap.S().Errorf("%v: %v", common.BriefHash(hash), err)
					}
					if isValid {
						zap.S().Debugf("Valid block %v has height %v, expecting %v", common.BriefHash(block.GetHash()), block.GetHeight(), height)
					} else {
						zap.S().Debugf("Invalid block %v has height %v, expecting %v", common.BriefHash(block.GetHash()), block.GetHeight(), height)
					}
					if !isValid {
						return err
					}
				}
				return nil
			})
			checkError(err)
		} else {
			zap.S().Debug("Will perform light validation of chain once connected.")
		}
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
		split := strings.Split(*olTestPeer, ":")
		port, err := strconv.ParseUint(split[1], 10, 16)
		checkError(err)
		allPeers.PeerList[*olTestPeer] = trHttp.Peer{IP: net.ParseIP(split[0]), Port: int(port)}
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

	blockChunkSize := 1000
	blockBuffer := make(map[string]*p2p_pb.BcBlock)
	connections := make(map[string]OverlinePeer)
	olMessages := make([]OverlineMessage, 0)
	dialer := net.Dialer{Timeout: time.Millisecond * 2000}
	for _, peer := range allPeers.PeerList {

		peerString := peer.IP.String() + fmt.Sprintf(":%d", peer.Port+1)

		zap.S().Infof("Working on peer: %v", peerString)

		tcpAddr, err := net.ResolveTCPAddr("tcp4", peerString)
		if err != nil {
			zap.S().Error(err)
			continue
		}

		conn, err := dialer.Dial("tcp4", tcpAddr.String())
		if err != nil {
			zap.S().Error(err)
			continue
		}

		messageHandler := OverlineMessageHandler{Messages: &olMessages, ID: id_bytes}

		messageHandler.Initialize(conn)

		go messageHandler.Run()

		go func() {
			blockStride := uint64(10)
			iStride := uint64(0)
			topRange := uint64(startingHeight + (iStride+1)*blockStride + 1)
			for messageHandler.Peer.Height == 0 || topRange < messageHandler.Peer.Height {
				low, high := uint64(iStride*blockStride), uint64((iStride+1)*blockStride)
				low += startingHeight + 1
				high += startingHeight + 1
				if high > topRange {
					high = topRange
				}
				reqstr := messages.GET_DATA + messages.SEPARATOR + fmt.Sprintf("%d%s%d", low, messages.SEPARATOR, high)
				reqLen := len(reqstr)
				request := make([]byte, reqLen+4)

				binary.BigEndian.PutUint32(request[0:], uint32(reqLen))
				copy(request[4:], []byte(reqstr))

				zap.S().Infof("Sending request: %v (topRange=%v, Peer Height=%v)", reqstr, topRange, messageHandler.Peer.Height)

				// write the request to the connection
				_, err = conn.Write(request)
				checkError(err)

				if iStride%10 == 0 {
					time.Sleep(time.Millisecond * 500)
				}
				iStride += 1
				topRange = uint64(startingHeight + (iStride+1)*blockStride + 1)
			}
		}()

		for {
			messageHandler.Mu.Lock()
			if len(*messageHandler.Messages) > 0 {
				zap.S().Infof("There are %v messages in the queue!", len(*messageHandler.Messages))
				for len(*messageHandler.Messages) > 0 {
					oneMessage := (*messageHandler.Messages)[0]
					*messageHandler.Messages = (*messageHandler.Messages)[1:]
					zap.S().Infof("Popped message of type %v from %v", oneMessage.Type, hex.EncodeToString(oneMessage.PeerID))
					switch oneMessage.Type {
					case messages.DATA:
						blocks := p2p_pb.BcBlocks{}
						err = proto.Unmarshal(oneMessage.Value, &blocks)
						checkError(err)
						for _, block := range blocks.Blocks {
							isValid, err := validation.IsValidBlock(block)
							if isValid {
								blockBuffer[block.GetHash()] = block
							} else {
								zap.S().Warnf("Not serializing invalid block %v: %v", common.BriefHash(block.GetHash()), err)
							}
						}
						if len(blockBuffer) < blockChunkSize {
							zap.S().Infof("Received DATA: Saved %v new blocks to buffer (%v)!", len(blocks.Blocks), len(blockBuffer))
						} else {
							blocks = p2p_pb.BcBlocks{}
							for hash, block := range blockBuffer {
								blocks.Blocks = append(blocks.Blocks, block)
								delete(blockBuffer, hash)
							}
							go SerializeBlocks(db, blocks)
							zap.S().Infof("Received DATA: Serialized %v new blocks!", len(blocks.Blocks))
						}
					case messages.BLOCK:
						b := p2p_pb.BcBlock{}
						err = proto.Unmarshal(oneMessage.Value, &b)
						checkError(err)
						isValid, err := validation.IsValidBlock(&b)
						if isValid {
							messageHandler.Peer.Height = b.GetHeight()
							zap.S().Infof("Received Valid BLOCK: Set Height of %v to %v", hex.EncodeToString(oneMessage.PeerID), b.GetHeight())
						} else {
							zap.S().Infof("Received Invalid BLOCK: %v -> %v", b.GetHeight(), err)
						}
					default:
						zap.S().Debugf("Throwing away: %v->%v", hex.EncodeToString(oneMessage.PeerID), oneMessage.Type)
					}
				}
			}
			messageHandler.Mu.Unlock()
			time.Sleep(time.Millisecond * 50)
		}

		time.Sleep(time.Second * 600)

		os.Exit(1)

		peerid, block, err := handshake(conn, id_bytes)
		if err == nil {
			zap.S().Infof("Completed handshake: %v -> %v @ %v", peerString, hex.EncodeToString(peerid), block.GetHeight())
			connections[peerString] = OverlinePeer{Conn: conn, ID: peerid, Height: block.GetHeight()}
		} else {
			zap.S().Infof("Failed to connect to %v: %v", peerString, err)
			conn.Close()
		}
	}

	zap.S().Infof("Successful handshakes with %d nodes!", len(connections))

	blockStride := uint64(10)
	bcBlocks := p2p_pb.BcBlocks{}
	for peerAddr, peer := range connections {
		zap.S().Infof("Successfully connected to %v", peerAddr)

		iBlockRange := uint64(0)
		zap.S().Info("sync request -> %v chunks", peer.Height/blockStride)
		for iBlockRange < 350 {
			zap.S().Infof("beep %v %v", iBlockRange*blockStride, (iBlockRange+1)*blockStride)
			blocks, err := getBlockRange(peer.Conn, iBlockRange*blockStride, (iBlockRange+1)*blockStride)
			checkError(err)
			for _, b := range blocks.Blocks {
				if b.GetHeight() < (iBlockRange+1)*blockStride && b.GetHeight() >= iBlockRange*blockStride {
					zap.S().Infof("Got Block %v: %v", b.GetHeight(), b.GetHash())
					bcBlocks.Blocks = append(bcBlocks.Blocks, b)
				}
			}
			iBlockRange += 1
			if len(bcBlocks.Blocks) >= blockChunkSize {
				SerializeBlocks(db, bcBlocks)
				bcBlocks = p2p_pb.BcBlocks{}
			}
		}

	}
	if len(bcBlocks.Blocks) > 0 {
		zap.S().Info("serializer tail catch!")
		SerializeBlocks(db, bcBlocks)
	}

	checkError(err)

	defer func() {
		for _, peer := range connections {
			peer.Conn.Close()
		}
	}()

	for {
		time.Sleep(time.Second * 1)
	}

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

func handshake(conn net.Conn, id []byte) ([]byte, p2p_pb.BcBlock, error) {
	buf := make([]byte, 0x1000)

	peerHandshake := make([]byte, 0)
	blockHandshake := make([]byte, 0)

	block := p2p_pb.BcBlock{}

	lenSize := binary.PutUvarint(buf, uint64(len(id)))

	copy(buf[lenSize:], id)

	zap.S().Debugf("handshake -> sending lenSize: %v id: %v", lenSize, hex.EncodeToString(buf[lenSize:len(id)]))

	_, err := conn.Write(buf[:lenSize+len(id)])
	if err != nil {
		return make([]byte, 0), block, err
	}

	// receive the full message and then decode it
	n, err := conn.Read(buf)
	if err != nil {
		return make([]byte, 0), block, err
	}
	// there are four possibilities for handshake replies:
	// 1 - the first read is the length of handshake (then we need to read more)
	// 2 - the first read is the length-encoded full handshake reply
	// 3 - the first read is the L-E full handshake reply and the handshake block
	// 4 - the first read	is the L-E full	handshake reply	and part of the handshake block
	zap.S().Debugf("peer handshake -> received buffer of length %d", n)

	// all cases deal with length of peer handshake (always one byte for sane peers)
	cursor := uint64(1)
	lenPeer, _ := binary.Uvarint(buf[:cursor])
	peerHandshake = append(peerHandshake, buf[:cursor]...)

	if lenPeer != 32 && lenPeer != 20 {
		return make([]byte, 0), block, errors.New(fmt.Sprintf("Invalid peer handshake length: %v", lenPeer))
	}

	// if we only received the first few bytes of the peer handshake
	// ask for more
	nPeerRemain := uint64(0)
	if uint64(n) < lenPeer+1 {
		zap.S().Debugf("handshake -> received partial peer handshake, requesting more data")
		nPeerRemain = lenPeer + 1 - uint64(n)
		n, err = conn.Read(buf[n:])
		if err != nil {
			return make([]byte, 0), block, err
		}
		zap.S().Debugf("peer handshake -> received buffer of length %d, expecting %v", n, nPeerRemain)
		peerHandshake = append(peerHandshake, buf[cursor:cursor+lenPeer]...)
	} else { // process the rest of buf
		peerHandshake = append(peerHandshake, buf[cursor:cursor+lenPeer]...)
	}
	zap.S().Debugf("peer handshake -> full handshake: (%v) %v", lenPeer, hex.EncodeToString(peerHandshake[cursor:cursor+lenPeer]))
	cursor += lenPeer

	// now we are assured that anything remaining in buf or coming on a Read
	// is the handshake block
	if n <= len(peerHandshake) {
		// we need to wait for the block handshake to arrive - reset cursor
		zap.S().Debugf("block handshake -> requesting more data resetting cursor")
		cursor = 0
		n, err = conn.Read(buf)
		if err != nil {
			return make([]byte, 0), p2p_pb.BcBlock{}, err
		}
		zap.S().Debugf("block handshake -> received buffer of length %d", n)
	}

	lenBlock := uint64(binary.BigEndian.Uint32(buf[cursor : cursor+4]))
	zap.S().Debugf("block handshake -> expect block of length %v", lenBlock)
	blockHandshake = append(blockHandshake, buf[cursor:cursor+4]...)
	cursor += 4

	zap.S().Debugf("block handshake -> gotten %v need to get %v (n=%v cursor=%v)", uint64(n)-cursor, lenBlock, n, cursor)
	// offset by the cursor and the encoded length
	if (uint64(n) - cursor + 1) < lenBlock {
		// get the block part out of the buffer
		blockHandshake = append(blockHandshake, buf[cursor:n]...)
		zap.S().Debugf("block handshake -> handshake buffer now %v", len(blockHandshake))
		// retreive the rest of the block
		ntot := uint64(n) - cursor
		for ntot < lenBlock {
			n, err = conn.Read(buf)
			if err != nil {
				return make([]byte, 0), p2p_pb.BcBlock{}, err
			}
			ntot += uint64(n)
			zap.S().Debugf("block handshake -> requested more data ntot=%v", ntot)
			blockHandshake = append(blockHandshake, buf[:n]...)
			zap.S().Debugf("block handshake -> handshake buffer now %v", len(blockHandshake))
		}
	} else {
		blockHandshake = append(blockHandshake, buf[cursor:cursor+lenBlock]...)
	}

	// now we know we have the handshake header + block in blockHandshake
	// and the format of the reply is HEADER SEP BLOCK
	parts := bytes.Split(blockHandshake[4:], []byte(messages.SEPARATOR))
	if len(parts) != 2 {
		return make([]byte, 0), block, errors.New(fmt.Sprintf("Malformed block handshake received - has %v parts", len(parts)))
	}
	if messages.BLOCK != string(parts[0]) {
		return make([]byte, 0), block, errors.New(fmt.Sprintf("Incorrect block handshake header %v != %v", messages.BLOCK, string(parts[0])))
	}
	// now we are likely safe to decode the block
	err = proto.Unmarshal(parts[1], &block)
	if err != nil {
		return make([]byte, 0), block, err
	}
	zap.S().Debugf("Block has height: %v", block.GetHeight())
	zap.S().Debugf("Got a block:\n%v", block)

	return peerHandshake[1:], block, nil
}

func getBlockRange(conn net.Conn, low uint64, high uint64) (p2p_pb.BcBlocks, error) {
	reqstr := messages.GET_DATA + messages.SEPARATOR + fmt.Sprintf("%d%s%d", low, messages.SEPARATOR, high)
	reqLen := len(reqstr)
	request := make([]byte, reqLen+4)
	for i, ibyte := range []byte(reqstr) {
		request[i+4] = ibyte
	}

	binary.BigEndian.PutUint32(request[0:], uint32(reqLen))

	zap.S().Infof("Sending request: %v = %v", reqstr, request)

	// write the request to the connection
	_, err := conn.Write(request)
	if err != nil {
		return p2p_pb.BcBlocks{}, err
	}

	//time.Sleep(time.Second * 1)

	// receive the full message and then decode it
	buf := make([]byte, 0x10000)
	reply := make([]byte, 0)
	n, err := conn.Read(buf)
	if err != nil {
		return p2p_pb.BcBlocks{}, err
	}
	zap.S().Infof("Initial read of block range: %v", n)
	reply = append(reply, buf[:n]...)
	nleft := int(binary.BigEndian.Uint32(reply[:4]))
	ntot := n - 4
	for ntot < nleft {
		n, err = conn.Read(buf)
		checkError(err)
		ntot += n
		reply = append(reply, buf[:n]...)
		zap.S().Debugf("Read additional bytes: %v %v %v", nleft, ntot, n)
	}

	zap.S().Debugf("Expect to read %v more bytes, received %v", nleft, ntot)
	zap.S().Debugf("Reply header: %v", string(reply[4:11]))
	zap.S().Debugf("Expected lengths: ntot=%v nleft=%v", ntot, nleft)

	parts := bytes.Split(reply[4:], []byte(messages.SEPARATOR))

	zap.S().Debugf("parts[0] : %v", string(parts[0]))
	zap.S().Debugf("Got %d blocks!", len(parts[1:]))

	blocks := p2p_pb.BcBlocks{}
	err = proto.Unmarshal(parts[1], &blocks)
	if err != nil {
		return blocks, err
	}

	zap.S().Debugf("received reply of length %v blocks", len(blocks.Blocks))

	return blocks, nil
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
