package p2p

import (
        dht "github.com/anacrolix/dht/v2"
        "github.com/anacrolix/dht/v2/krpc"
        "github.com/anacrolix/log"
        "github.com/anacrolix/torrent/metainfo"
        "github.com/anacrolix/torrent/tracker"
        trHttp "github.com/anacrolix/torrent/tracker/http"
        "net"
        "net/http"
	"crypto/sha1"
)

const ID_LENGTH = 32

type PeeringManager struct {
  enableDiscovery bool
  trackers []string
  id []bytes
  idHash metainfo.Hash
  networkHash metainfo.Hash
}

func NewPeeringManager(
  enableDiscovery bool,
  trackers []string,
  id string,
  network string,
  ) (*PeeringManager, error) {
  
  idBytes := make([]byte, ID_LENGTH)
  copy(id_bytes, id)
  if len(id) < ID_LENGTH {
    rand.Read(id_bytes[len(id):])
  }

  idHash := metainfo.HashBytes(id_bytes)
  networkHash := metainfo.HashBytes([]byte(network))

  pm := PeeringManager{
    enableDiscovery,
    trackers,
    idBytes,
    idHash,
    networkHash,
  }

  pm.init()

  return &pm
}

func (pm *PeeringManager) init() {

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
        zap.S().Debugf("Checking the following trackers for peers: %v", pm.trackers)

        allPeers := ConcurrentPeerMap{PeerList: make(map[string]trHttp.Peer)}
        if pm.enableDiscovery {
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