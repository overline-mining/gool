package main

import (
  "context"
  "flag"
  "time"

  "github.com/wavesplatform/gowaves/pkg/util/fdlimit"
  "github.com/wavesplatform/gowaves/pkg/libs/ntptime"
  "github.com/wavesplatform/gowaves/pkg/types"
  
  "github.com/overline-mining/gool/src/common"

  "fmt"

  "go.uber.org/zap"

	"github.com/xgfone/bt/metainfo"
	"github.com/xgfone/bt/tracker"
  "github.com/xgfone/bt/dht"
  "net"
	"strconv"
	"sync"
  "strings"
)
  
var (
  logLevel = flag.String("log-level", "INFO", "Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
  blockchainType = flag.String("blockchain-type", "mainnet", "Blockchain type: mainnet/testnet/integration")
  limitAllConnections = flag.Uint("limit-connections", 60, "Total limit of network connections, both inbound and outbound. Divided in half to limit each direction. Default value is 60.")
  dbFileDescriptors = flag.Int("db-file-descriptors", 500, "Maximum allowed file descriptors count that will be used by state database. Default value is 500.")
)

func getPeersFromTrackers(id, infohash metainfo.Hash, trackers []string) (peers []string) {
	c, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
  
	resp := tracker.GetPeers(c, id, infohash, trackers)
	for _, r := range resp {
		for _, addr := range r.Resp.Addresses {
			addrs := addr.String()
			nonexist := true
			for _, peer := range peers {
				if peer == addrs {
					nonexist = false
					break
				}
			}

			if nonexist {
				peers = append(peers, addrs)
			}
		}
	}

	return
}

type testPeerManager struct {
	lock  sync.RWMutex
	peers map[metainfo.Hash][]metainfo.Address
}

func newTestPeerManager() *testPeerManager {
	return &testPeerManager{peers: make(map[metainfo.Hash][]metainfo.Address)}
}

func (pm *testPeerManager) AddPeer(infohash metainfo.Hash, addr metainfo.Address) {
	pm.lock.Lock()
	var exist bool
	for _, orig := range pm.peers[infohash] {
		if orig.Equal(addr) {
			exist = true
			break
		}
	}
	if !exist {
		pm.peers[infohash] = append(pm.peers[infohash], addr)
	}
	pm.lock.Unlock()
}

func (pm *testPeerManager) GetPeers(infohash metainfo.Hash, maxnum int,
	ipv6 bool) (addrs []metainfo.Address) {
	// We only supports IPv4, so ignore the ipv6 argument.
	pm.lock.RLock()
	_addrs := pm.peers[infohash]
	if _len := len(_addrs); _len > 0 {
		if _len > maxnum {
			_len = maxnum
		}
		addrs = _addrs[:_len]
	}
	pm.lock.RUnlock()
	return
}

func onSearch(infohash string, ip net.IP, port uint16) {
	addr := net.JoinHostPort(ip.String(), strconv.FormatUint(uint64(port), 10))
	fmt.Printf("%s is searching %s\n", addr, infohash)
}

func onTorrent(infohash string, ip net.IP, port uint16) {
	addr := net.JoinHostPort(ip.String(), strconv.FormatUint(uint64(port), 10))
	fmt.Printf("%s has downloaded %s\n", addr, infohash)
}

func newDHTServer(id metainfo.Hash, addr string, pm dht.PeerManager) (s *dht.Server, err error) {
	conn, err := net.ListenPacket("udp", addr)
	if err == nil {
		c := dht.Config{ID: id, PeerManager: pm, OnSearch: onSearch, OnTorrent: onTorrent}
		s = dht.NewServer(conn, c)
	}
	return
}
    
func main() {
  flag.Parse()

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

  ctx, cancel := context.WithCancel(context.Background())

  ntpTime, err := getNtp(ctx)
	if err != nil {
		zap.S().Error(err)
		cancel()
		return
	}

  zap.S().Info(ntpTime)

  id := metainfo.NewRandomHash()
  // bcnode mainnet infohash
  infoHash := metainfo.NewHashFromHexString("716ca5c568509b3652a21dd076e1bf842583267c")
          
  trackers := []string{
    "udp://tds-r3.blockcollider.org:16060/announce",
  	"udp://104.207.130.112:16060/announce",
    "udp://139.180.221.213:16060/announce",
    "udp://52.208.110.140:16060/announce",
    "udp://internal.xeroxparc.org:16060/announce",
    "udp://reboot.alcor1436.com:16060/announce",
    "udp://sa1.alcor1436.com:16060/announce",
  }

  peers := getPeersFromTrackers(id, infoHash, trackers)
	if len(peers) == 0 {
		fmt.Println("no peers")
		return
	}

  zap.S().Info(peers)

  pm := newTestPeerManager()
  server, err := newDHTServer(id, ":16061", pm)
  if err != nil {
		fmt.Println(err)
		return
	}
	defer server.Close()

  go server.Run()

  //wait for server to start
  time.Sleep(time.Second)

  peersnoport := []string{}
  for _, peer := range peers {
    peersnoport = append(peersnoport, strings.Split(peer, ":")[0])
  }

  server.Bootstrap([]string{
    "router.bittorrent.com:6881",
    "router.utorrent.com:6881",
    "dht.transmissionbt.com:6881",
    "router.bitcomet.com:6881",
    "dht.aelitis.com:6881",
    "18.210.15.44:16061",
  })

  zap.S().Info("Waiting for bootstrap to complete...")
  time.Sleep(time.Second * 30)
  zap.S().Info("Done waiting...")

  
  for _, peer := range peersnoport {
    pm.AddPeer(infoHash, metainfo.NewAddress(net.ParseIP(peer), 16061))
  }
    
  zap.S().Info(peers)

  // wait for DHT table to build from bootstrap
  time.Sleep(time.Second * 5)
  
  zap.S().Infof("server: %v", server.Node4Num())

  server.GetPeers(infoHash, func(r dht.Result) {
    if len(r.Peers) == 0 {
      fmt.Printf("no peers for %s\n", infoHash)
    } else {
      for _, peer := range r.Peers {
        fmt.Printf("%s: %s\n", infoHash, peer.String())
      }
    }
  })

  time.Sleep(time.Second * 2)

  server.GetPeers(infoHash, func(r dht.Result) {
    if len(r.Peers) == 0 {
      fmt.Printf("no peers for %s\n", infoHash)
    } else {
      for _, peer := range r.Peers {
        fmt.Printf("%s: %s\n", infoHash, peer.String())
      }
    }
  })

  time.Sleep(time.Second * 60)
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