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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	gor_mux "github.com/gorilla/mux"
	jsonrpc2http "go.neonxp.dev/jsonrpc2/http"
	jsonrpc2 "go.neonxp.dev/jsonrpc2/rpc"

	geth_rpc "github.com/ethereum/go-ethereum/rpc"
	chain "github.com/overline-mining/gool/src/blockchain"
	"github.com/overline-mining/gool/src/genesis"
	"github.com/overline-mining/gool/src/protocol/messages"
	"github.com/overline-mining/gool/src/protocol/services"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"github.com/overline-mining/gool/src/rpc"
	//"github.com/overline-mining/gool/src/transactions"
	"github.com/autom8ter/dagger"
	db "github.com/overline-mining/gool/src/database"
	"github.com/overline-mining/gool/src/validation"
	probar "github.com/schollz/progressbar/v3"
	"google.golang.org/protobuf/proto"
)

var (
	logLevel            = flag.String("log-level", "INFO", "Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	blockchainType      = flag.String("blockchain-type", "mainnet", "Blockchain type: mainnet/testnet/integration")
	limitAllConnections = flag.Uint("limit-connections", 60, "Total limit of network connections, both inbound and outbound. Divided in half to limit each direction. Default value is 60.")
	dbFileDescriptors   = flag.Int("db-file-descriptors", 500, "Maximum allowed file descriptors count that will be used by state database. Default value is 500.")
	olTestPeer          = flag.String("test-peer", "", "Specify an exact peer (ip:port) to connect to instead of using tracker")
	ipBlackList         = flag.String("black-list", "", "Specify a comma separated list of IPs to never connect to")
	ipWhiteList         = flag.String("white-list", "", "Specify a comma separated list of IPs we should only connect to")
	olWorkDir           = flag.String("ol-workdir", ".overline", "Specify the working directory for the node")
	dbFileName          = flag.String("db-file-name", "overline.boltdb", "The file we're going to write the blockchain to within olWorkDir")
	validateFullChain   = flag.Bool("full-validation", false, "Run a slow but complete validation of your local blockchain DB")
	dropChainstate      = flag.Bool("drop-chainstate", false, "Delete the last saved chainstate from the database")
	pruneDatabaseTo     = flag.Int("prune-database-to", 0, "Before starting the node, remove all blocks after this height.")
	ipcDisable          = flag.Bool("ipcdisable", false, "Turn off IPC based JSON-RPC.")
	ipcPath             = flag.String("ipcpath", ".overline/gool.sock", "Path to the IPC based JSON-RPC endpoint.")
	httpEnable          = flag.Bool("http", false, "Enable JSON-RPC over HTTP.")
	httpAddr            = flag.String("http.addr", "localhost", "The address bound to the HTTP JSON-RPC server.")
	httpPort            = flag.Int("http.port", 4712, "The port to listen on for the HTTP JSON-RPC server.")
	httpVhosts          = flag.String("http.vhosts", "", "allowed vhosts for accessing HTTP JSON-RPC server.")
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
	Conn      net.Conn
	Height    uint64
	ID        []byte
	Connected bool
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
	buf      [0xa00000]byte // 10 MB buffer for work
	buflen   int
}

func (mh *OverlineMessageHandler) Initialize(conn net.Conn, starter, genesis *p2p_pb.BcBlock) {
	//mh.Mu.Lock()
	//defer mh.Mu.Unlock()

	mh.buflen = 0

	peer_id, nbuf, err := handshake_peer(conn, mh.ID, starter, genesis, &mh.buf)
	zap.S().Debugf("handshake (%v) -> %v, %v, %v", conn.RemoteAddr(), hex.EncodeToString(peer_id), nbuf, err)
	mh.buflen = nbuf
	if nbuf == -1 || err != nil {
		zap.S().Errorf("Failed to connect to %v: %v", hex.EncodeToString(peer_id), err)
		conn.Close()
		return
	}
	zap.S().Infof("Completed handshake: %v -> %v", common.BriefHash(hex.EncodeToString(mh.ID)), common.BriefHash(hex.EncodeToString(peer_id)))
	mh.Peer = OverlinePeer{Conn: conn, ID: peer_id, Height: 0, Connected: true}
}

type IBDWork struct {
	nRequests uint64
	isRetry   bool
}

type IBDWorkList struct {
	Mu            sync.Mutex
	AllowedBlocks map[uint64]IBDWork
}

func (mh *OverlineMessageHandler) Run() {
	messageCounter := uint64(0)
	for mh.buflen == -1 {
		time.Sleep(time.Millisecond * 100)
	}
	for {
		n := 0
		cursor := 0
		currentMessage := make([]byte, 0)
		var err error
		if mh.buflen < 4 {
			n, err = mh.Peer.Conn.Read(mh.buf[mh.buflen:])
			if err != nil {
				zap.S().Warnf("closing peer %v: %v", hex.EncodeToString(mh.Peer.ID), err)
				mh.Peer.Connected = false
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
					zap.S().Warnf("closing peer %v: %v", hex.EncodeToString(mh.Peer.ID), err)
					mh.Peer.Connected = false
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

	// setup blacklist
	badPeers := make(map[string]bool)
	if len(*ipBlackList) > 0 {
		for _, badPeer := range strings.Split(*ipBlackList, ",") {
			zap.S().Debugf("Adding %v to blacklist", badPeer)
			badPeers[badPeer] = true
		}
	}

	// setup whitelist
	goodPeers := make(map[string]bool)
	if len(*ipWhiteList) > 0 {
		for _, goodPeer := range strings.Split(*ipWhiteList, ",") {
			zap.S().Debugf("Adding %v to whitelist", goodPeer)
			goodPeers[goodPeer] = true
		}
	}

	// database testing
	startingHeight := uint64(0)
	dbFilePath := filepath.Join(*olWorkDir, *dbFileName)
	gooldb := db.OverlineDB{Config: db.DefaultOverlineDBConfig()}

	pruneToHeight := uint64(0)
	if *pruneDatabaseTo > 0 {
		pruneToHeight = uint64(*pruneDatabaseTo)
	}
	err = gooldb.Open(dbFilePath, *dropChainstate, pruneToHeight)

	// add ctrl-c catcher for closing the database
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		gooldb.Close()
		os.Exit(1)
	}()

	if err != nil {
		startingHeight = 1
		zap.S().Warn("Blockchain file was uninitialized - creating genesis block")
		var gblock *p2p_pb.BcBlock
		gblock, err = genesis.BuildGenesisBlock(*olWorkDir)
		if err != nil {
			zap.S().Fatal(err)
			os.Exit(1)
		}
		_, err = gooldb.AddBlock(gblock)
	}

	genesisBlock, err := gooldb.GetBlockByHeight(uint64(1))
	if err != nil {
		zap.S().Panic("Couldn't get genesis block!")
	}

	startingHighestBlock := gooldb.HighestBlock()
	startingHeight = startingHighestBlock.GetHeight()

	defer gooldb.Close()

	checkError(err)

	// run full local validation if requested
	if *validateFullChain {
		gooldb.FullLocalValidation()
	}

	// start the gool ingestion thread
	gooldb.Run()

	ibdWorkList := IBDWorkList{AllowedBlocks: make(map[uint64]IBDWork)}

	// create the chain ingester
	goolChain := chain.OverlineBlockchain{
		Config:                           chain.DefaultOverlineBlockchainConfig(),
		BlockGraph:                       dagger.NewGraph(),
		DB:                               &gooldb,
		Heads:                            make(map[string]bool),
		HeadsToCheck:                     make(map[string]uint64),
		IbdTransitionPeriodRelativeDepth: float64(0.005),
	}
	goolChain.UnsetFollowingChain()

	rpcServer := geth_rpc.NewServer()

	blockchainService := new(rpc.BlockchainService)
	blockchainService.Chain = &goolChain

	// olNodeRPC interface
	olNode_GetBlockHash := func(ctx context.Context, hash *[]string) (*p2p_pb.BcBlock, error) {
		return blockchainService.GetBlockByHash((*hash)[0])
	}

	olNodeService := jsonrpc2http.New()
	olNodeService.Register("getBlockHash", jsonrpc2.Wrap(olNode_GetBlockHash))

	adminService := new(rpc.AdminService)

	rpcServer.RegisterName("ovl", blockchainService)
	rpcServer.RegisterName("admin", adminService)

	var ipcRpcListener net.Listener
	if !*ipcDisable {
		ipcRpcListener, _ = net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: *ipcPath})
		defer ipcRpcListener.Close()
		go rpcServer.ServeListener(ipcRpcListener)
	}

	if *httpEnable {
		go func() {
			err := http.ListenAndServe(fmt.Sprintf("%v:%v", *httpAddr, *httpPort), rpc.NewHTTPHandlerStack(rpcServer, []string{}, []string{*httpVhosts}, []byte{}))
			zap.S().Error(err)
		}()
	}

	// the olNodeService binds to 0.0.0.0 and exposes a minimum number of interfaces
	// necessary to interact with javascript olnodes
	go func() {
		r := gor_mux.NewRouter()
		r.Handle("/rpc", olNodeService)
		err := http.ListenAndServe(":3000", r)
		zap.S().Error(err)
	}()

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
		"udp://104.207.130.112:16060/announce",
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
							ipString := fmt.Sprintf("%v", peer.IP)
							if _, ok := badPeers[ipString]; !ok {
								if _, ok = goodPeers[ipString]; ok || len(goodPeers) == 0 {
									allPeers.AddPeer(fmt.Sprintf("%v", peer), peer)
								} else {
									zap.S().Debugf("Skipping peer %v not in white-list", peer.IP)
								}
							} else {
								zap.S().Debugf("Skipping black-listed peer %v", peer.IP)
							}
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
	dhtConfig.Conn, err = net.ListenPacket("udp4", "0.0.0.0:16060")
	if err != nil {
		zap.S().Info(err)
		return
	}
	dhtConfig.Logger.FilterLevel(log.Debug)
	dhtConfig.OnQuery = func(query *krpc.Msg, source net.Addr) (propagate bool) {
		zap.S().Debugf("Received query (%v): %v", source, query)
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

	// server for incoming connections
	go func() {
		nodeListen, err := net.Listen("tcp4", ":16061")
		if err != nil {
			zap.S().Panic(err)
		}
		defer nodeListen.Close()
		zap.S().Infof("Node server listening on %v", nodeListen.Addr())
		for {
			conn, err := nodeListen.Accept()
			if err != nil {
				zap.S().Panic(err)
			}
			zap.S().Infof("New connection request %v -> %v", conn.RemoteAddr(), conn.LocalAddr())
			conn_ip := strings.Split(conn.RemoteAddr().String(), ":")[0]
			if _, ok := badPeers[conn_ip]; ok {
				conn.Close()
				continue
			}
			if _, ok := goodPeers[conn_ip]; !ok && len(goodPeers) > 0 {
				conn.Close()
				continue
			}
			handler := OverlineMessageHandler{Mu: &olMessageMu, Messages: &olMessages, ID: id_bytes}
			handler.Initialize(conn, startingHighestBlock, genesisBlock)
			olHandlerMapMu.Lock()
			if _, ok := olMessageHandlers[hex.EncodeToString(handler.Peer.ID)]; !ok && len(handler.Peer.ID) > 0 && bytes.Compare(handler.ID, handler.Peer.ID) != 0 {
				go handler.Run()
				if _, ok := olMessageHandlers[hex.EncodeToString(handler.Peer.ID)]; !ok {
					olMessageHandlers[hex.EncodeToString(handler.Peer.ID)] = handler
				}
				olHandlerMapMu.Unlock()
			} else {
				conn.Close()
				olHandlerMapMu.Unlock()
			}
		}
	}()

	// discover outgoing peer connections
	//var waitForPeers sync.WaitGroup
	for _, peer := range allPeers.PeerList {
		//waitForPeers.Add(1)
		go func() {
			//defer waitForPeers.Done()

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
			handler.Initialize(conn, startingHighestBlock, genesisBlock)
			if len(handler.Peer.ID) > 0 && bytes.Compare(handler.ID, handler.Peer.ID) != 0 {
				go handler.Run()
				olHandlerMapMu.Lock()
				if _, ok := olMessageHandlers[hex.EncodeToString(handler.Peer.ID)]; !ok {
					olMessageHandlers[hex.EncodeToString(handler.Peer.ID)] = handler
				}
				olHandlerMapMu.Unlock()
			} else {
				conn.Close()
			}

		}()
		time.Sleep(time.Millisecond * 10)
	}

	//waitForPeers.Wait()
	time.Sleep(time.Second * 2)

	olHandlerMapMu.Lock()
	zap.S().Infof("Successful handshakes with %d nodes!", len(olMessageHandlers))
	olHandlerMapMu.Unlock()

	// setup a loop to remove disconnected peers from the messageHandler
	go func() {
		for {
			time.Sleep(time.Second * 10)
			olHandlerMapMu.Lock()
			for peer, handler := range olMessageHandlers {
				if !handler.Peer.Connected {
					zap.S().Infof("Expunging disconnected peer %v %v", common.BriefHash(peer), handler.Peer.Conn.RemoteAddr())
					delete(olMessageHandlers, peer)
				}
			}
			olHandlerMapMu.Unlock()
		}
	}()

	ibdBar := probar.Default(int64(startingHeight+1), "initial block download ->")
	ibdBar.Add64(int64(startingHeight))

	go func() {
		for {
			heightDbl := float64(gooldb.SerializedHeight())
			thresholdDbl := heightDbl + float64(gooldb.Config.AncientChunkSize)
			lowestNonZeroPeerHeight := float64(0)
			highestNonZeroPeerHeight := uint64(0)

			localHandlers := make(map[string]OverlineMessageHandler)
			olHandlerMapMu.Lock()
			for peer, messageHandler := range olMessageHandlers {
				localHandlers[peer] = messageHandler
			}
			olHandlerMapMu.Unlock()

			for _, msgHandler := range localHandlers {
				peerHeight := msgHandler.Peer.Height
				floatHeight := float64(peerHeight)
				if floatHeight > 0 && (floatHeight < lowestNonZeroPeerHeight || lowestNonZeroPeerHeight == float64(0)) {
					lowestNonZeroPeerHeight = floatHeight
				}
				if peerHeight > highestNonZeroPeerHeight {
					highestNonZeroPeerHeight = peerHeight
				}
			}

			if !goolChain.IsFollowingChain() && lowestNonZeroPeerHeight > 0.0 && lowestNonZeroPeerHeight*(1-goolChain.IbdTransitionPeriodRelativeDepth) < thresholdDbl {
				goolChain.SetFollowingChain()
			}
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
						if err == nil && len(blocks.Blocks) > 0 {
							zap.S().Debugf("Got blocklist of length %v with starting value: %v %v", len(blocks.Blocks), blocks.Blocks[0].GetHeight(), blocks.Blocks[0].GetHash())
							/*
								for _, blk := range blocks.Blocks {
								   zap.S().Infof("Block %v @ %v", blk.GetHash(), blk.GetHeight())
								}
								zap.S().Infof("Starting height %v, ending height %v", blocks.Blocks[0].GetHeight(), blocks.Blocks[len(blocks.Blocks)-1].GetHeight())
							*/
						}
						if gooldb.IsInitialBlockDownload() {
							goodBlocks := p2p_pb.BcBlocks{}
							ibdWorkList.Mu.Lock()
							highestReceivedBlock := uint64(0)
							for _, block := range blocks.Blocks {
								if thework, ok := ibdWorkList.AllowedBlocks[block.GetHeight()]; ok {
									thework.nRequests -= 1
									if block.GetHeight() > highestReceivedBlock {
										highestReceivedBlock = block.GetHeight()
									}
									goodBlocks.Blocks = append(goodBlocks.Blocks, block)
									ibdWorkList.AllowedBlocks[block.GetHeight()] = thework
									if thework.nRequests == 0 {
										delete(ibdWorkList.AllowedBlocks, block.GetHeight())
									}
								}
							}
							ibdWorkList.Mu.Unlock()
							blocks = goodBlocks
							if highestNonZeroPeerHeight != 0 && highestNonZeroPeerHeight-20 < highestReceivedBlock {
								gooldb.UnSetInitialBlockDownload()
							}
						}
						if err == nil {
							added := 0
							if goolChain.IsFollowingChain() {
								added = goolChain.AddBlockRange(&blocks)
							} else if gooldb.IsInitialBlockDownload() {
								added, _ = gooldb.AddBlockRange(&blocks)
							}
							if gooldb.IsInitialBlockDownload() {
								ibdBar.Add(added)
							}
						} else {
							zap.S().Error(err)
						}
					case messages.BLOCK:
						b := new(p2p_pb.BcBlock)
						err = proto.Unmarshal(oneMessage.Value, b)
						if err != nil {
							zap.S().Errorf("Deserialization error in BLOCK: %v", err)
							continue
						}
						isValid, err := validation.IsValidBlock(b)
						if isValid {
							peerIDHex := hex.EncodeToString(oneMessage.PeerID)
							olHandlerMapMu.Lock()
							msgHandler := olMessageHandlers[peerIDHex]
							oldHeight := msgHandler.Peer.Height
							msgHandler.Peer.Height = b.GetHeight()
							lclAddr := msgHandler.Peer.Conn.LocalAddr()
							rmtAddr := msgHandler.Peer.Conn.RemoteAddr()
							olMessageHandlers[peerIDHex] = msgHandler

							olHandlerMapMu.Unlock()
							if oldHeight == 0 {
								zap.S().Infof("Setting known height of peer %v (%v) to %v", common.BriefHash(peerIDHex), rmtAddr, b.GetHeight())
							} else {
								zap.S().Debugf("Received Valid BLOCK: Set Height of %v to %v (%v <- %v)", common.BriefHash(peerIDHex), b.GetHeight(), lclAddr, rmtAddr)
							}
							if goolChain.IsFollowingChain() {
								added := goolChain.AddBlock(b)
								if added && !gooldb.IsInitialBlockDownload() {
									//highest, err := goolChain.GetHighestBlock()
									if err != nil {
										olHandlerMapMu.Lock()
										for aPeerID, aHandler := range olMessageHandlers {
											if aPeerID != peerIDHex && aHandler.Peer.Height > 0 && aHandler.Peer.Height < b.GetHeight() {
												if msgHandler.Peer.Connected {
													sendBlockBytes(msgHandler.Peer.Conn, oneMessage.Value)
													checkError(err)
												}
											}
										}
										olHandlerMapMu.Unlock()
									}
								}
							}
							if gooldb.IsInitialBlockDownload() {
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
					case messages.GET_BLOCK:
						peerIDHex := hex.EncodeToString(oneMessage.PeerID)
						olHandlerMapMu.Lock()
						msgHandler := olMessageHandlers[peerIDHex]
						olHandlerMapMu.Unlock()

						highestBlock, err := goolChain.GetHighestBlock()
						if err != nil {
							zap.S().Error(err)
							continue
						}

						olHandlerMapMu.Lock()
						msgHandler = olMessageHandlers[peerIDHex]
						if msgHandler.Peer.Connected {
							err = sendBlock(msgHandler.Peer.Conn, highestBlock)
							if err != nil {
								zap.S().Error(err)
							}
						}
						olHandlerMapMu.Unlock()

					case messages.GET_DATA:
						peerIDHex := hex.EncodeToString(oneMessage.PeerID)
						olHandlerMapMu.Lock()
						msgHandler := olMessageHandlers[peerIDHex]
						lclAddr := msgHandler.Peer.Conn.LocalAddr()
						rmtAddr := msgHandler.Peer.Conn.RemoteAddr()
						olHandlerMapMu.Unlock()
						blockRange := bytes.Split(oneMessage.Value, []byte(messages.SEPARATOR))
						low := uint64(0)
						high := uint64(0)
						highestBlock, err := goolChain.GetHighestBlock()
						var lowStr, highStr string
						if len(blockRange) == 2 {
							var lowErr error = nil
							var highErr error = nil
							lowStr = strings.Trim(string(blockRange[0]), "\"")
							highStr = strings.Trim(string(blockRange[1]), "\"")
							low, lowErr = strconv.ParseUint(lowStr, 10, 64)
							high, highErr = strconv.ParseUint(highStr, 10, 64)
							if lowErr != nil || highErr != nil {
								zap.S().Error("Could not parse given block range: %v -> %v", blockRange[0], blockRange[1])
								continue
							}
							if high > 10*low {
								if highStr[len(highStr)-2:] == "16" {
									zap.S().Debug("Trimming errant trailing 16 from abnormal request")
									high, _ = strconv.ParseUint(highStr[:len(highStr)-2], 10, 64)
									zap.S().Debugf("High side of request is now: %v", high)
								} else if highStr[len(highStr)-1:] == "1" {
									zap.S().Debug("Trimming errant trailing 1 from abnormal request")
									high, _ = strconv.ParseUint(highStr[:len(highStr)-1], 10, 64)
									zap.S().Debugf("High side of request is now: %v", high)
								}
							}
						}
						zap.S().Debugf("GET_DATA %v -> %v-%v (%v -> %v)", peerIDHex, low, high, lclAddr, rmtAddr)
						go func() {
							if err != nil {
								zap.S().Error(err)
								return
							}
							if highestBlock.GetHeight() < low {
								zap.S().Warnf("Low range of request %v exceeds local chain height %v", low, highestBlock.GetHeight())
								low = high + 1

							} else {
								if highestBlock.GetHeight() < high {
									zap.S().Warnf("Truncating request %v to highest available block: %v", high, highestBlock.GetHeight())
									high = highestBlock.GetHeight()
								}
								if high-low > 55 {
									zap.S().Warnf("Requested block range is length %v, reducing to length 55 (original request strings %v -> %v)", high-low, lowStr, highStr)
									high = low + 55
								}
							}

							// collect the necessary blocks from the database
							blocksToSend := new(p2p_pb.BcBlocks)
							for i := low; i <= high; i++ {
								block, err := goolChain.GetBlockByHeight(i)
								if err != nil {
									zap.S().Errorf("GET_DATA Error -> %v", err)
									return
								}
								blocksToSend.Blocks = append(blocksToSend.Blocks, block)
							}
							bytesToSend, err := proto.Marshal(blocksToSend)
							if (len(blocksToSend.Blocks) > 0 || low > high) && err == nil {
								reqbytes := []byte(messages.DATA)
								reqbytes = append(reqbytes, []byte(messages.SEPARATOR)...)
								reqbytes = append(reqbytes, bytesToSend...)
								reqLen := len(reqbytes)
								request := make([]byte, reqLen+4)
								copy(request[4:], reqbytes)
								binary.BigEndian.PutUint32(request[0:], uint32(reqLen))

								olHandlerMapMu.Lock()
								msgHandler := olMessageHandlers[peerIDHex]
								if olMessageHandlers[peerIDHex].Peer.Connected {
									n, err := msgHandler.Peer.Conn.Write(request)
									if n != len(request) || err != nil {
										msgHandler.Peer.Conn.Close()
										zap.S().Warnf("Didn't write complete request to outbound connection, marking connection as closed! (%v)", err)
										handler := olMessageHandlers[peerIDHex]
										handler.Peer.Connected = false
										olMessageHandlers[peerIDHex] = handler
									} else {
										zap.S().Debugf("GET_DATA -> Wrote %v bytes / %v blocks to the outbound connection!", n, len(blocksToSend.Blocks))
									}
								}
								olHandlerMapMu.Unlock()
							} else {
								zap.S().Errorf("Error replying to GET_DATA: %v", err)
							}
						}()
					case messages.GET_RECORD:
						peerIDHex := hex.EncodeToString(oneMessage.PeerID)
						olHandlerMapMu.Lock()
						msgHandler := olMessageHandlers[peerIDHex]
						lclAddr := msgHandler.Peer.Conn.LocalAddr()
						rmtAddr := msgHandler.Peer.Conn.RemoteAddr()
						olHandlerMapMu.Unlock()
						zap.S().Infof("GET_RECORD %v -> %v", lclAddr, rmtAddr)
					case messages.RECORD:
						peerIDHex := hex.EncodeToString(oneMessage.PeerID)
						olHandlerMapMu.Lock()
						msgHandler := olMessageHandlers[peerIDHex]
						lclAddr := msgHandler.Peer.Conn.LocalAddr()
						rmtAddr := msgHandler.Peer.Conn.RemoteAddr()
						olHandlerMapMu.Unlock()
						zap.S().Infof("RECORD %v <- %v", lclAddr, rmtAddr)
					case messages.GET_CONFIG:
						peerIDHex := hex.EncodeToString(oneMessage.PeerID)
						olHandlerMapMu.Lock()
						msgHandler := olMessageHandlers[peerIDHex]
						lclAddr := msgHandler.Peer.Conn.LocalAddr()
						rmtAddr := msgHandler.Peer.Conn.RemoteAddr()
						zap.S().Infof("GET_CONFIG %v <- %v", lclAddr, rmtAddr)

						configToSend := new(p2p_pb.Config)
						configToSend.Version = 0x01 // fix me legacy versioning jsnode stuff

						uiService := new(p2p_pb.Service)
						uiService.Version = 0x01 // fixme legacy versioning jsnode stuff
						uiService.Uuid = services.BORDERLESS_RPC
						uiService.Text = "BC_UI_PORT:3000"
						configToSend.Services = append(configToSend.Services, uiService)

						p2pService := new(p2p_pb.Service)
						p2pService.Version = 0x01 // fix me legacy versioning jsnode stuff
						p2pService.Uuid = services.AT_P2P
						p2pService.Text = strings.Join(messages.AllMessageTypes, ",")
						configToSend.Services = append(configToSend.Services, p2pService)

						bytesToSend, err := proto.Marshal(configToSend)

						if err == nil {
							reqbytes := []byte(messages.CONFIG)
							reqbytes = append(reqbytes, []byte(messages.SEPARATOR)...)
							reqbytes = append(reqbytes, bytesToSend...)
							reqLen := len(reqbytes)
							request := make([]byte, reqLen+4)
							copy(request[4:], reqbytes)
							binary.BigEndian.PutUint32(request[0:], uint32(reqLen))

							if olMessageHandlers[peerIDHex].Peer.Connected {
								n, err := msgHandler.Peer.Conn.Write(request)
								if n != len(request) || err != nil {
									msgHandler.Peer.Conn.Close()
									zap.S().Warnf("Didn't write complete request to outbound connection, marking connection as closed! (%v)", err)
									handler := olMessageHandlers[peerIDHex]
									handler.Peer.Connected = false
									olMessageHandlers[peerIDHex] = handler
								} else {
									zap.S().Debugf("GET_CONFIG -> Wrote %v bytes blocks to the outbound connection!", n)
								}
							}
						}

						olHandlerMapMu.Unlock()

					default:
						zap.S().Warnf("Throwing away: %v -> %v", common.BriefHash(hex.EncodeToString(oneMessage.PeerID)), oneMessage.Type)
					}

				}
				olMessageMu.Unlock()
			} else {
				olMessageMu.Unlock()
				time.Sleep(time.Millisecond * 100)
			}
		}
	}()

	// heartbeat GETBLOCK
	go func() {
		for {
			time.Sleep(time.Second * 30)
			olHandlerMapMu.Lock()
			for peer, messageHandler := range olMessageHandlers {
				reqstr := messages.GET_BLOCK
				reqLen := len(reqstr)
				request := make([]byte, reqLen+4)
				binary.BigEndian.PutUint32(request[0:], uint32(reqLen))
				copy(request[4:], []byte(reqstr))

				if messageHandler.Peer.Connected {
					// write the request to the connection
					n, err := messageHandler.Peer.Conn.Write(request)
					//zap.S().Debugf("Wrote %v bytes to the outbound connection!", n)
					if n != len(request) || err != nil {
						messageHandler.Peer.Conn.Close()
						zap.S().Warn("Didn't write complete request to outbound connection, marking connection as closed! (%v)", err)
						handler := olMessageHandlers[peer]
						handler.Peer.Connected = false
						olMessageHandlers[peer] = handler
					} else {
						zap.S().Debugf("HEARTBEAT: %v -> %v", common.BriefHash(peer), reqstr)
					}
				}
			}
			olHandlerMapMu.Unlock()
		}

	}()

	// FIXME multiplexing peers only really works with a proper download manager
	gooldb.UnSetMultiplexPeers()

	go func(goolChain *chain.OverlineBlockchain) {
		blockStride := uint64(20)
		iStride := uint64(1) // start from 1
		topRange := uint64(startingHeight + (iStride)*blockStride + 1)
		nMaxRetries := 10      // maximum number of retries for a given 1000 block chunk, if we fail to retry this many times we exit ibd
		nRetriesThisChunk := 0 // number of retries on this chunk
		for {
			needsRetry := false
			if nRetriesThisChunk > nMaxRetries {
				zap.S().Warn("Maximum retries for blockchunk reached, leaving initial blockdownload!")
				gooldb.UnSetInitialBlockDownload()
				time.Sleep(time.Second * 1)
			}
			if !gooldb.IsInitialBlockDownload() {
				break
			}

			// FIXME for later switch between multiplexing peers
			/*
				if nRetriesThisChunk == 0 {
					gooldb.SetMultiplexPeers()
				} else {
					gooldb.UnSetMultiplexPeers()
				}
			*/

			olHandlerMapMu.Lock()
			localHandlers := make(map[string]OverlineMessageHandler)
			nConnectedPeers := 0
			for peer, messageHandler := range olMessageHandlers {
				localHandlers[peer] = messageHandler
				if messageHandler.Peer.Height > 0 {
					nConnectedPeers++
				}
			}
			olHandlerMapMu.Unlock()
			if nConnectedPeers == 0 {
				time.Sleep(time.Second * 1)
				continue
			}
			for _, messageHandler := range localHandlers {
				high := uint64(iStride*blockStride) + startingHeight + 1
				if messageHandler.Peer.Height > 0 && high < messageHandler.Peer.Height && messageHandler.Peer.Height-high <= uint64(gooldb.Config.AncientChunkSize) {
					gooldb.UnSetMultiplexPeers()
				}
			}
			for peer, messageHandler := range localHandlers {
				olMessageMu.Lock()
				low, high := uint64((iStride-1)*blockStride), uint64(iStride*blockStride)
				low += startingHeight + 1
				high += startingHeight + 1
				zap.S().Debugf("Peer %v height = %v low = %v high = %v", common.BriefHash(peer), messageHandler.Peer.Height, low, high)
				if messageHandler.Peer.Height == 0 || messageHandler.Peer.Height < low || messageHandler.Peer.Height < topRange {
					olMessageMu.Unlock()
					continue
				}
				if high > messageHandler.Peer.Height {
					high = messageHandler.Peer.Height
				}
				ibdWorkList.Mu.Lock()
				for i := low; i < high; i++ {
					if _, ok := ibdWorkList.AllowedBlocks[i]; ok {
						nrequests := uint64(1) //ibdWorkList.AllowedBlocks[i].nRequests + 1
						ibdWorkList.AllowedBlocks[i] = IBDWork{nRequests: nrequests, isRetry: nRetriesThisChunk > 0}
					} else {
						ibdWorkList.AllowedBlocks[i] = IBDWork{nRequests: uint64(1), isRetry: nRetriesThisChunk > 0}
					}
				}
				ibdWorkList.Mu.Unlock()
				reqstr := messages.GET_DATA + messages.SEPARATOR + fmt.Sprintf("%d%s%d", low, messages.SEPARATOR, high)
				reqLen := len(reqstr)
				request := make([]byte, reqLen+4)
				binary.BigEndian.PutUint32(request[0:], uint32(reqLen))
				copy(request[4:], []byte(reqstr))

				// write the request to the connection
				if messageHandler.Peer.Connected {
					n, err := messageHandler.Peer.Conn.Write(request)
					//zap.S().Debugf("Wrote %v bytes to the outbound connection!", n)
					if n != len(request) || err != nil {
						messageHandler.Peer.Conn.Close()
						zap.S().Warn("Didn't write complete request to outbound connection, skipping! (%v)", err)

					} else {
						zap.S().Debugf("Sending request: %v -> %v", peer, reqstr)
					}
				}
				olMessageMu.Unlock()
				time.Sleep(time.Millisecond * 50) // pause between outbound requests

				if gooldb.IsMultiplexPeers() {
					if iStride%50 == 0 {
						break //always make sure we're waiting for proper-sized chunks
					}
					iStride += 1
				}

				topRange = uint64(startingHeight + (iStride+1)*blockStride + 1)
			}

			if iStride%50 == 0 && iStride != 0 { // if we've submitted a request for 1000 blocks - wait until we have received them all
				zap.S().Debugf("Submitted block requests for range [%v,%v] (%v)", startingHeight+(iStride-50)*blockStride+1, startingHeight+(iStride)*blockStride+1, iStride)
				ibdTimeout := 10 // 5s in chunks of 500 ms, if we wait this long then we re-try the last 1000 blocks
				for {
					ibdWorkList.Mu.Lock()
					nBlocksRemaining := len(ibdWorkList.AllowedBlocks)
					ibdWorkList.Mu.Unlock()
					if nBlocksRemaining == 0 { // chunk is done
						nRetriesThisChunk = 0
						break
					} else {
						if ibdTimeout == 0 {
							needsRetry = true
							break
						} else {
							ibdTimeout -= 1
						}
						zap.S().Debugf("waiting for %v blocks to arrive...", nBlocksRemaining)
						/*
						   ibdWorkList.Mu.Lock()
						   for height, num := range ibdWorkList.AllowedBlocks {
						     zap.S().Debugf("\twaiting for block %v - %v", height, num)
						   }
						   ibdWorkList.Mu.Unlock()
						*/
						time.Sleep(time.Millisecond * 500)
					}
				}
			}

			if needsRetry {
				iStride -= 49
				nRetriesThisChunk++
				ibdWorkList.Mu.Lock()
				// reset ibd work list so we don't keep waiting...
				for height, _ := range ibdWorkList.AllowedBlocks {
					delete(ibdWorkList.AllowedBlocks, height)
				}
				ibdWorkList.Mu.Unlock()
				zap.S().Warnf("Retrying block requests for range [%v,%v] (%v/%v)", startingHeight+(iStride-1)*blockStride+1, startingHeight+(iStride+49)*blockStride+1, nRetriesThisChunk, nMaxRetries)
			}
			if !needsRetry && !gooldb.IsMultiplexPeers() {
				iStride += 1
			}
			time.Sleep(time.Millisecond * 350)
		}
		zap.L().Info("Initial block download has completed.")
		highestBlock, err := goolChain.GetHighestBlock()
		if err == nil {
			olHandlerMapMu.Lock()
			for _, msgHandler := range olMessageHandlers {
				if msgHandler.Peer.Connected {
					err = sendBlock(msgHandler.Peer.Conn, highestBlock)
					if err != nil {
						zap.S().Error(err)
					}
				}
			}
			for peer, msgHandler := range olMessageHandlers {
				if msgHandler.Peer.Connected {
					reqstr := messages.GET_BLOCK
					reqLen := len(reqstr)
					request := make([]byte, reqLen+4)
					binary.BigEndian.PutUint32(request[0:], uint32(reqLen))
					copy(request[4:], []byte(reqstr))

					zap.S().Infof("Sending request: %v -> %v", hex.EncodeToString(msgHandler.Peer.ID), reqstr)

					// write the request to the connection
					n, err := msgHandler.Peer.Conn.Write(request)
					//zap.S().Debugf("Wrote %v bytes to the outbound connection!", n)
					if n != len(request) {
						msgHandler.Peer.Conn.Close()
						zap.S().Warn("Didn't write complete request to outbound connection! (%v)", err)
						handler := olMessageHandlers[peer]
						handler.Peer.Connected = false
						olMessageHandlers[peer] = handler

					}
				}
			}
			olHandlerMapMu.Unlock()
		} else {
			zap.S().Fatal("Unable to get highest block after initial block download!")
		}
		for {
			olHandlerMapMu.Lock()
			localHandlers := make(map[string]OverlineMessageHandler)
			for peer, messageHandler := range olMessageHandlers {
				if messageHandler.Peer.Connected {
					localHandlers[peer] = messageHandler
				}
			}
			olHandlerMapMu.Unlock()
			headsToCheck := make(map[string]uint64)
			goolChain.CheckupMu.Lock()
			for hash, height := range goolChain.HeadsToCheck {
				headsToCheck[hash] = height
				zap.S().Infof("Going to check history of disjoint block %v @ %v", common.BriefHash(hash), height)
				delete(goolChain.HeadsToCheck, hash)
			}
			goolChain.CheckupMu.Unlock()
			// send a request to all connected nodes for the 10 previous blocks
			// to the head height to check
			for _, headHeight := range headsToCheck {
				for peer, messageHandler := range localHandlers {
					olMessageMu.Lock()
					low, high := headHeight-11, headHeight-1
					if messageHandler.Peer.Height == 0 || messageHandler.Peer.Height < low || messageHandler.Peer.Height < topRange {
						olMessageMu.Unlock()
						continue
					}
					if high > messageHandler.Peer.Height {
						high = messageHandler.Peer.Height
					}
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
				}
			}
			time.Sleep(time.Millisecond * 1000)

		}

	}(&goolChain)

	defer func() {
		olHandlerMapMu.Lock()
		for _, handler := range olMessageHandlers {
			if handler.Peer.Connected {
				handler.Peer.Conn.Close()
			}
		}
		olHandlerMapMu.Unlock()
	}()

	for {
		time.Sleep(time.Second * 1)
	}
	return
}

func sendBlock(conn net.Conn, block *p2p_pb.BcBlock) error {
	bytesToSend, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	return sendBlockBytes(conn, bytesToSend)
}

func sendBlockBytes(conn net.Conn, blockBytes []byte) error {
	reqbytes := []byte(messages.BLOCK)
	reqbytes = append(reqbytes, []byte(messages.SEPARATOR)...)
	reqbytes = append(reqbytes, blockBytes...)
	reqLen := len(reqbytes)
	request := make([]byte, reqLen+4)
	copy(request[4:], reqbytes)
	binary.BigEndian.PutUint32(request[0:], uint32(reqLen))

	n, err := conn.Write(request)
	if n != len(request) {
		conn.Close()
		zap.S().Warn("Didn't write complete request to outbound connection!")
	} else {
		zap.S().Debugf("sendBlock -> Wrote %v bytes to the outbound connection!", n)
	}
	if err != nil {
		return err
	}
	return nil
}

func handshake_peer(conn net.Conn, id []byte, starter, genesis *p2p_pb.BcBlock, buf *[0xa00000]byte) ([]byte, int, error) {
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

	// now that we have made the first part of the handshake, send current highest block
	starterBytes, err := proto.Marshal(starter)
	if err != nil {
		return make([]byte, 0), -1, err
	}
	/*
	   	recBytes := []byte(messages.GET_RECORD)
	   	recLen := len(recBytes)
	   	recuest := make([]byte, recLen+4)
	   	copy(recuest[4:], recBytes)
	   	binary.BigEndian.PutUint32(recuest[0:], uint32(recLen))


	   	n, err = conn.Write(recuest)
	           //zap.S().Debugf("Wrote %v bytes to the outbound connection!", n)
	           if n != len(recuest) {
	                   zap.S().Fatal("Fatal error: didn't write complete request to outbound connection!")
	                   os.Exit(1)
	           }
	           checkError(err)
	*/
	reqbytes := []byte(messages.BLOCK)
	reqbytes = append(reqbytes, []byte(messages.SEPARATOR)...)
	reqbytes = append(reqbytes, starterBytes...)
	reqLen := len(reqbytes)
	request := make([]byte, reqLen+4)
	copy(request[4:], reqbytes)
	binary.BigEndian.PutUint32(request[0:], uint32(reqLen))

	n, err = conn.Write(request)
	//zap.S().Debugf("Wrote %v bytes to the outbound connection!", n)
	if n != len(request) {
		zap.S().Fatal("Fatal error: didn't write complete request to outbound connection!")
		os.Exit(1)
	}
	checkError(err)

	return peerHandshake[1 : lenPeer+1], int(ntot - int(cursor)), nil
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
