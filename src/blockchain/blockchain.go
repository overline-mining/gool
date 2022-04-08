package blockchain

import (
	"github.com/autom8ter/dagger"
	"github.com/overline-mining/gool/src/coin"
	"github.com/overline-mining/gool/src/common"
	"github.com/overline-mining/gool/src/database"
	//"github.com/overline-mining/gool/src/validation"
	"encoding/hex"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"go.uber.org/zap"
	"math/big"
	"sort"
	"sync"
)

const (
	BLOCK_TYPE      = "block"
	CONNECTION_TYPE = "connects"
)

type OverlineBlockchainConfig struct {
	DisjointCheckupDepth int `json:"disjoint_checkup_depth"` // if there is a chain segment not connected to db, how long does it have to be to ask to go look for blocks to try to connect?
}

func DefaultOverlineBlockchainConfig() OverlineBlockchainConfig {
	return OverlineBlockchainConfig{
		DisjointCheckupDepth: 10,
	}
}

type OverlineBlockchain struct {
	Config                           OverlineBlockchainConfig
	Mu                               sync.Mutex
	CheckupMu                        sync.Mutex
	followMu                         sync.Mutex
	Heads                            map[string]bool
	HeadsToCheck                     map[string]uint64
	BlockGraph                       *dagger.Graph
	DB                               *database.OverlineDB
	isFollowingChain                 bool
	IbdTransitionPeriodRelativeDepth float64
}

func (obc *OverlineBlockchain) SetFollowingChain() {
	obc.followMu.Lock()
	obc.isFollowingChain = true
	obc.followMu.Unlock()
}

func (obc *OverlineBlockchain) UnsetFollowingChain() {
	obc.followMu.Lock()
	obc.isFollowingChain = false
	obc.followMu.Unlock()
}

func (obc *OverlineBlockchain) IsFollowingChain() bool {
	obc.followMu.Lock()
	defer obc.followMu.Unlock()
	return obc.isFollowingChain
}

func (obc *OverlineBlockchain) connectToChildBlock(from, to dagger.Node) error {
	_, err := obc.BlockGraph.SetEdge(
		from.Path,
		to.Path,
		dagger.Node{
			Path: dagger.Path{
				XType: CONNECTION_TYPE,
			},
			Attributes: map[string]interface{}{},
		})
	return err
}

func (obc *OverlineBlockchain) AddBlock(block *p2p_pb.BcBlock) {
	_, found := obc.BlockGraph.GetNode(
		dagger.Path{
			XID:   block.GetHash(),
			XType: BLOCK_TYPE,
		})
	if found { // no need to process a block that is already in the graph
		return
	}
	// check if the block is already in the db, if so pass
	if !obc.DB.IsInitialBlockDownload() {
		testHashBytes, err := hex.DecodeString(block.GetHash())
		var testDbBlock *p2p_pb.BcBlock
		if err == nil {
			testDbBlock, err = obc.DB.GetBlock(testHashBytes)
		}
		if err == nil && testDbBlock.GetHash() == block.GetHash() {
			return
		}
	}
	// create the node in the block graph
	blkNode := obc.BlockGraph.SetNode(
		dagger.Path{
			XID:   block.GetHash(),
			XType: BLOCK_TYPE,
		},
		map[string]interface{}{
			"block": block,
		})
	// get the parent node if it exists
	parentNode, found := obc.BlockGraph.GetNode(
		dagger.Path{
			XID:   block.GetPreviousHash(),
			XType: BLOCK_TYPE,
		})
	if found {
		zap.S().Debugf(
			"found previous hash in chain graph %v -> %v %v",
			common.BriefHash(block.GetPreviousHash()),
			common.BriefHash(parentNode.Attributes["block"].(*p2p_pb.BcBlock).GetHash()),
			found,
		)
		err := obc.connectToChildBlock(parentNode, blkNode)
		if err != nil {
			zap.S().Panic(err)
		}
	} else {
		zap.S().Infof(
			"Could not find previous hash %v for new block %v @ %v",
			common.BriefHash(block.GetPreviousHash()),
			common.BriefHash(block.GetHash()),
			block.GetHeight(),
		)
		obc.Mu.Lock()
		obc.Heads[block.GetHash()] = true
		obc.Mu.Unlock()
	}
	// copying is faster than locking on the outer loop of the DFS
	obc.Mu.Lock()
	heads := make(map[string]bool)
	for h, v := range obc.Heads {
		heads[h] = v
	}
	obc.Mu.Unlock()
	// clean up the chain state:
	// - remove heads (and connected blocks) that are less than serialized height
	// - connect heads that may have arrived out of order
	serHeight := obc.DB.SerializedHeight()
	for h, _ := range heads {
		headNode, found := obc.BlockGraph.GetNode(
			dagger.Path{
				XID:   h,
				XType: BLOCK_TYPE,
			})
		if !found {
			zap.S().Panicf("Could not find head block in graph %v", common.BriefHash(h))
		}
		headBlock := headNode.Attributes["block"].(*p2p_pb.BcBlock)
		if headBlock.GetHeight() < serHeight {
			// make new heads out of children
			obc.BlockGraph.RangeEdgesFrom(CONNECTION_TYPE, headNode.Path, func(e dagger.Edge) bool {
				zap.S().Infof("pop head to %v", e.To.XID)
				heads[e.To.XID] = true
				return true
			})
			obc.BlockGraph.DelNode(headNode.Path)
			delete(heads, h)
			continue
		}
		// get the head's parent node if it exists
		parentNode, found := obc.BlockGraph.GetNode(
			dagger.Path{
				XID:   headNode.Attributes["block"].(*p2p_pb.BcBlock).GetPreviousHash(),
				XType: BLOCK_TYPE,
			})
		if found {
			zap.S().Debugf(
				"found previous hash in chain graph for head %v -> %v %v",
				common.BriefHash(headBlock.GetHash()),
				common.BriefHash(headBlock.GetPreviousHash()),
				common.BriefHash(parentNode.Attributes["block"].(*p2p_pb.BcBlock).GetHash()),
			)
			err := obc.connectToChildBlock(parentNode, headNode)
			if err != nil {
				zap.S().Panic(err)
			}
			// remove the now-integrated head from the list of heads
			delete(heads, h)
		}
	}

	// go over all heads
	// if head's chain is longer than maturity and connected to database
	// pop the head off that chain, update to new head(s)
	// add the highest popped head to the chain
	// if the head is longer than the checkup depth and not connected to database
	// add to checkup list and grab blocks from all to see if it can be sorted out
	for {
		poppedHeadsMap := make(map[string]*p2p_pb.BcBlock)
		poppedHeads := make([]*p2p_pb.BcBlock, 0)
		for hash, _ := range heads {
			headNode, found := obc.BlockGraph.GetNode(
				dagger.Path{
					XID:   hash,
					XType: BLOCK_TYPE,
				})
			if !found {
				zap.S().Panicf("Could not find head node %v", common.BriefHash(hash))
			}
			highestBlock := headNode.Attributes["block"].(*p2p_pb.BcBlock)
			obc.BlockGraph.DFS(CONNECTION_TYPE, headNode.Path, func(node dagger.Node) bool {
				blk := node.Attributes["block"].(*p2p_pb.BcBlock)
				if blk.GetHeight() > highestBlock.GetHeight() {
					highestBlock = blk
				}
				return true
			})
			headBlock := headNode.Attributes["block"].(*p2p_pb.BcBlock)
			hashBytes, err := hex.DecodeString(headBlock.GetPreviousHash())
			var dbBlock *p2p_pb.BcBlock
			if err == nil {
				dbBlock, err = obc.DB.GetBlock(hashBytes)
			}
			if highestBlock != nil {
				hasDB := false
				if err == nil && headBlock.GetPreviousHash() == dbBlock.GetHash() {
					hasDB = true
					zap.S().Infof(
						"DB << (%v): %s -> %s has highest block: (%v): %v and length %v",
						headBlock.GetHeight(),
						common.BriefHash(headBlock.GetHash()),
						common.BriefHash(headBlock.GetPreviousHash()),
						highestBlock.GetHeight(),
						common.BriefHash(highestBlock.GetHash()),
						highestBlock.GetHeight()-headBlock.GetHeight(),
					)
				} else {
					zap.S().Infof(
						"      (%v): %s -> %s has highest block: (%v): %v and length %v",
						headBlock.GetHeight(),
						common.BriefHash(headBlock.GetHash()),
						common.BriefHash(headBlock.GetPreviousHash()),
						highestBlock.GetHeight(),
						common.BriefHash(highestBlock.GetHash()),
						highestBlock.GetHeight()-headBlock.GetHeight(),
					)
				}
				blockDepth := highestBlock.GetHeight() - headBlock.GetHeight()
				if blockDepth > coin.COINBASE_MATURITY && headBlock.GetPreviousHash() == dbBlock.GetHash() {
					// make new heads out of children
					obc.BlockGraph.RangeEdgesFrom(CONNECTION_TYPE, headNode.Path, func(e dagger.Edge) bool {
						heads[e.To.XID] = true
						return true
					})
					obc.BlockGraph.DelNode(headNode.Path)
					delete(heads, hash)
					poppedHeadsMap[headBlock.GetHash()] = headBlock
				}
				_, poppedParent := poppedHeadsMap[headBlock.GetPreviousHash()]
				if !obc.DB.IsInitialBlockDownload() && !(hasDB || poppedParent) && (blockDepth > uint64(obc.Config.DisjointCheckupDepth)) {
					obc.CheckupMu.Lock()
					if _, ok := obc.HeadsToCheck[headBlock.GetHash()]; !ok {
						obc.HeadsToCheck[headBlock.GetHash()] = headBlock.GetHeight()
					}
					obc.CheckupMu.Unlock()
				}
			}

		}

		for _, blk := range poppedHeadsMap {
			poppedHeads = append(poppedHeads, blk)
		}

		//finally stick the best popped head in the database, the others perish
		sort.SliceStable(poppedHeads, func(i, j int) bool {
			if poppedHeads[i].GetHeight() == poppedHeads[j].GetHeight() {
				iDist, _ := new(big.Int).SetString(poppedHeads[i].GetTotalDistance(), 10)
				jDist, _ := new(big.Int).SetString(poppedHeads[j].GetTotalDistance(), 10)
				compare := iDist.Cmp(jDist)
				if compare == 0 {
					return poppedHeads[i].GetTimestamp() < poppedHeads[j].GetTimestamp()
				}
				return compare < 0
			}
			return poppedHeads[i].GetHeight() < poppedHeads[j].GetHeight()
		})

		if len(poppedHeads) > 0 {
			zap.S().Infof("Popping block %v %v to the database!", poppedHeads[len(poppedHeads)-1].GetHeight(), common.BriefHash(poppedHeads[len(poppedHeads)-1].GetHash()))
			obc.DB.AddBlock(poppedHeads[len(poppedHeads)-1])
		}

		// copy the updated heads list back to the lock-protected version
		obc.Mu.Lock()
		obc.Heads = make(map[string]bool)
		for h, v := range heads {
			obc.Heads[h] = v
		}
		obc.Mu.Unlock()

		if len(poppedHeads) == 0 {
			break
		}

	}
}

func (obc *OverlineBlockchain) AddBlockRange(blocks *p2p_pb.BcBlocks) {
	for _, block := range blocks.Blocks {
		obc.AddBlock(block)
	}
}
