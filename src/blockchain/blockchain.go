package blockchain

import (
	"github.com/autom8ter/dagger"
	"github.com/overline-mining/gool/src/coin"
	"github.com/overline-mining/gool/src/common"
	"github.com/overline-mining/gool/src/database"
	//"github.com/overline-mining/gool/src/validation"
	"encoding/hex"
	"errors"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"go.uber.org/zap"
	"sort"
	"sync"
)

const (
	BLOCK_TYPE      = "block"
	CONNECTION_TYPE = "connects"
)

type HeadInformation struct {
	Hash         string
	Depth        uint64
	LowestBlock  *p2p_pb.BcBlock
	HighestBlock *p2p_pb.BcBlock
	HasDB        bool
}

type ProgressInfo struct {
	Done                   bool
	StartingBlock          *p2p_pb.BcBlock
	CurrentBlock           *p2p_pb.BcBlock
	HighestPeerBlockHeight uint64
}

type OverlineBlockchainConfig struct {
	DisjointCheckupDepth  int `json:"disjoint_checkup_depth"`  // if there is a chain segment not connected to db, how long does it have to be to ask to go look for blocks to try to connect?
	DisjointCheckupHeight int `json:"disjoint_checkup_height"` // if any chain segment not connected to db is higher by this number of blocks, investigate it
	DisplayDepth          int `json:"display_depth"`           // only show chains of this length or longer
}

func DefaultOverlineBlockchainConfig() OverlineBlockchainConfig {
	return OverlineBlockchainConfig{
		DisjointCheckupDepth:  10,
		DisjointCheckupHeight: 20,
		DisplayDepth:          10,
	}
}

type OverlineBlockchain struct {
	Config                           OverlineBlockchainConfig
	Mu                               sync.Mutex
	GraphMu                          sync.Mutex
	CheckupMu                        sync.Mutex
	followMu                         sync.Mutex
	Heads                            map[string]bool
	HeadsToCheck                     map[string]uint64
	currentHighestBlock              *p2p_pb.BcBlock
	currentHighestNode               *dagger.Node
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

func (obc *OverlineBlockchain) AddBlock(block *p2p_pb.BcBlock) bool {
	_, found := obc.BlockGraph.GetNode(
		dagger.Path{
			XID:   block.GetHash(),
			XType: BLOCK_TYPE,
		})
	if found { // no need to process a block that is already in the graph
		return false
	}
	// check if the block is already in the db, if so pass
	if !obc.DB.IsInitialBlockDownload() {
		testHashBytes, err := hex.DecodeString(block.GetHash())
		var testDbBlock *p2p_pb.BcBlock
		if err == nil {
			testDbBlock, err = obc.DB.GetBlockByHash(testHashBytes)
		}
		if err == nil && testDbBlock.GetHash() == block.GetHash() {
			return false
		}
	}
	// create the node in the block graph
	obc.GraphMu.Lock()
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
	obc.GraphMu.Unlock()
	if found {
		zap.S().Debugf(
			"found previous hash in chain graph %v -> %v %v",
			common.BriefHash(block.GetPreviousHash()),
			common.BriefHash(parentNode.Attributes["block"].(*p2p_pb.BcBlock).GetHash()),
			found,
		)
		obc.GraphMu.Lock()
		err := obc.connectToChildBlock(parentNode, blkNode)
		obc.GraphMu.Unlock()
		if err != nil {
			zap.S().Panic(err)
		}
	} else {
		if !obc.DB.IsInitialBlockDownload() {
			zap.S().Infof(
				"Could not find previous hash %v for new block %v @ %v",
				common.BriefHash(block.GetPreviousHash()),
				common.BriefHash(block.GetHash()),
				block.GetHeight(),
			)
		}
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
		obc.GraphMu.Lock()
		headNode, found := obc.BlockGraph.GetNode(
			dagger.Path{
				XID:   h,
				XType: BLOCK_TYPE,
			})
		obc.GraphMu.Unlock()
		if !found {
			zap.S().Panicf("Could not find head block in graph %v", common.BriefHash(h))
		}
		headBlock := headNode.Attributes["block"].(*p2p_pb.BcBlock)
		if headBlock.GetHeight() < serHeight {
			// make new heads out of children
			obc.GraphMu.Lock()
			obc.BlockGraph.RangeEdgesFrom(CONNECTION_TYPE, headNode.Path, func(e dagger.Edge) bool {
				if !obc.DB.IsInitialBlockDownload() {
					zap.S().Infof("pop head to %v", e.To.XID)
				}
				heads[e.To.XID] = true
				return true
			})
			obc.BlockGraph.DelNode(headNode.Path)
			obc.GraphMu.Unlock()
			delete(heads, h)
			continue
		}
		// get the head's parent node if it exists
		obc.GraphMu.Lock()
		parentNode, found := obc.BlockGraph.GetNode(
			dagger.Path{
				XID:   headNode.Attributes["block"].(*p2p_pb.BcBlock).GetPreviousHash(),
				XType: BLOCK_TYPE,
			})
		obc.GraphMu.Unlock()
		if found {
			zap.S().Debugf(
				"found previous hash in chain graph for head %v -> %v %v",
				common.BriefHash(headBlock.GetHash()),
				common.BriefHash(headBlock.GetPreviousHash()),
				common.BriefHash(parentNode.Attributes["block"].(*p2p_pb.BcBlock).GetHash()),
			)
			obc.GraphMu.Lock()
			err := obc.connectToChildBlock(parentNode, headNode)
			obc.GraphMu.Unlock()
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
	// mark the head with the longest chain and the head with the highest chain
	longestHeadWithDB := HeadInformation{Hash: "", Depth: uint64(0), LowestBlock: nil, HighestBlock: nil, HasDB: false}
	highestHead := HeadInformation{Hash: "", Depth: uint64(0), LowestBlock: nil, HighestBlock: nil, HasDB: false}
	for {
		poppedHeadsMap := make(map[string]*p2p_pb.BcBlock)
		poppedHeads := make([]*p2p_pb.BcBlock, 0)
		if !obc.DB.IsInitialBlockDownload() {
			zap.S().Infof("There are %v chain heads to follow! Showing chains longer than %v", len(heads), obc.Config.DisplayDepth)
		}
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
			obc.GraphMu.Lock()
			obc.BlockGraph.DFS(CONNECTION_TYPE, headNode.Path, func(node dagger.Node) bool {
				blk := node.Attributes["block"].(*p2p_pb.BcBlock)
				if common.BlockOrderingRule(highestBlock, blk) {
					highestBlock = blk
				}
				return true
			})
			obc.GraphMu.Unlock()
			headBlock := headNode.Attributes["block"].(*p2p_pb.BcBlock)
			hashBytes, err := hex.DecodeString(headBlock.GetPreviousHash())
			var dbBlock *p2p_pb.BcBlock
			if err == nil {
				dbBlock, err = obc.DB.GetBlockByHash(hashBytes)
			}
			if highestBlock != nil {
				hasDB := false
				chainLength := highestBlock.GetHeight() - headBlock.GetHeight()
				if err == nil && headBlock.GetPreviousHash() == dbBlock.GetHash() {
					hasDB = true
					if chainLength > uint64(obc.Config.DisplayDepth) {
						if !obc.DB.IsInitialBlockDownload() {
							zap.S().Infof(
								"DB << (%v): %s -> %s has highest block: (%v): %v and length %v",
								headBlock.GetHeight(),
								common.BriefHash(headBlock.GetHash()),
								common.BriefHash(headBlock.GetPreviousHash()),
								highestBlock.GetHeight(),
								common.BriefHash(highestBlock.GetHash()),
								highestBlock.GetHeight()-headBlock.GetHeight(),
							)
						}
					}
				} else {
					if chainLength > uint64(obc.Config.DisplayDepth) {
						if !obc.DB.IsInitialBlockDownload() {
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
					}
				}
				blockDepth := highestBlock.GetHeight() - headBlock.GetHeight()
				if blockDepth > coin.COINBASE_MATURITY && headBlock.GetPreviousHash() == dbBlock.GetHash() {
					// make new heads out of children
					obc.GraphMu.Lock()
					obc.BlockGraph.RangeEdgesFrom(CONNECTION_TYPE, headNode.Path, func(e dagger.Edge) bool {
						heads[e.To.XID] = true
						return true
					})
					obc.BlockGraph.DelNode(headNode.Path)
					obc.GraphMu.Unlock()
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
				if (hasDB || poppedParent) &&
					(longestHeadWithDB.LowestBlock == nil ||
						common.BlockOrderingRule(longestHeadWithDB.LowestBlock, headBlock) ||
						blockDepth > longestHeadWithDB.Depth) {
					longestHeadWithDB.Hash = hash
					longestHeadWithDB.Depth = blockDepth
					longestHeadWithDB.LowestBlock = headBlock
					longestHeadWithDB.HighestBlock = highestBlock
					longestHeadWithDB.HasDB = true
				}

				if highestHead.LowestBlock == nil || common.BlockOrderingRule(highestHead.HighestBlock, highestBlock) {
					highestHead.Hash = hash
					highestHead.Depth = blockDepth
					highestHead.LowestBlock = headBlock
					highestHead.HighestBlock = highestBlock
					highestHead.HasDB = (hasDB || poppedParent)
				}
			}

		}

		if longestHeadWithDB.LowestBlock != nil {
			obc.Mu.Lock()
			if common.BlockOrderingRule(obc.currentHighestBlock, longestHeadWithDB.HighestBlock) {
				obc.currentHighestBlock = longestHeadWithDB.HighestBlock
			}
			obc.Mu.Unlock()
			if highestHead.LowestBlock != nil && !highestHead.HasDB {
				heightDiff := highestHead.HighestBlock.GetHeight() - longestHeadWithDB.HighestBlock.GetHeight()
				if heightDiff > uint64(obc.Config.DisjointCheckupHeight) {
					obc.CheckupMu.Lock()
					obc.HeadsToCheck[highestHead.LowestBlock.GetHash()] = highestHead.LowestBlock.GetHeight()
					obc.CheckupMu.Unlock()
				}
			}
		}

		for _, blk := range poppedHeadsMap {
			poppedHeads = append(poppedHeads, blk)
		}

		//finally stick the best popped head in the database, the others perish
		sort.SliceStable(poppedHeads, func(i, j int) bool {
			return common.BlockOrderingRule(poppedHeads[i], poppedHeads[j])
		})

		if len(poppedHeads) > 0 {
			if !obc.DB.IsInitialBlockDownload() {
				zap.S().Infof("Popping block %v %v to the database!", poppedHeads[len(poppedHeads)-1].GetHeight(), common.BriefHash(poppedHeads[len(poppedHeads)-1].GetHash()))
			}
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
	return true
}

func (obc *OverlineBlockchain) AddBlockRange(blocks *p2p_pb.BcBlocks) int {
	added := 0
	for _, block := range blocks.Blocks {
		if obc.AddBlock(block) {
			added++
		}
	}
	return added
}

func (obc *OverlineBlockchain) GetBlockByHash(hash string) (*p2p_pb.BcBlock, error) {
	var block *p2p_pb.BcBlock = nil
	var err error = nil
	var hashBytes []byte
	obc.GraphMu.Lock()
	node, found := obc.BlockGraph.GetNode(
		dagger.Path{
			XID:   hash,
			XType: BLOCK_TYPE,
		})
	obc.GraphMu.Unlock()
	if found {
		block = node.Attributes["block"].(*p2p_pb.BcBlock)
	} else {
		hashBytes, err = hex.DecodeString(hash)
		if err != nil {
			return block, err
		}
		block, err = obc.DB.GetBlockByHash(hashBytes)
	}
	return block, err
}

func (obc *OverlineBlockchain) GetBlockByHeight(height uint64) (*p2p_pb.BcBlock, error) {
	var block *p2p_pb.BcBlock = nil
	var err error = nil

	obc.Mu.Lock()
	defer obc.Mu.Unlock()
	highestBlock := obc.currentHighestBlock

	if highestBlock != nil {
		highestHash := highestBlock.GetHash()
		highestHeight := highestBlock.GetHeight()

		if highestHeight == height {
			return obc.currentHighestBlock, nil
		}

		if highestHeight < height {
			return block, errors.New("Requested height exceeds chain height!")
		}

		obc.GraphMu.Lock()
		highestNode, _ := obc.BlockGraph.GetNode(
			dagger.Path{
				XID:   highestHash,
				XType: BLOCK_TYPE,
			})
		// walk back from the current best block and find the block with requested height
		// ignore other paths in tree
		obc.BlockGraph.ReverseDFS(CONNECTION_TYPE, highestNode.Path, func(node dagger.Node) bool {
			blk := node.Attributes["block"].(*p2p_pb.BcBlock)
			if blk.GetHeight() == height {
				zap.S().Debugf("Found height in block tree: %v == %v", blk.GetHeight(), height)
				block = blk
			}
			return true
		})
		obc.GraphMu.Unlock()
	}
	if block == nil {
		block, err = obc.DB.GetBlockByHeight(height)
	}
	return block, err
}

func (obc *OverlineBlockchain) GetBlockByTx(txHash string) (*p2p_pb.BcBlock, error) {
	var block *p2p_pb.BcBlock = nil
	var err error = nil

	txHashBytes, err := hex.DecodeString(txHash)

	if err != nil {
		return block, err
	}

	obc.Mu.Lock()
	highestHash := obc.currentHighestBlock.GetHash()
	obc.Mu.Unlock()

	obc.GraphMu.Lock()
	highestNode, _ := obc.BlockGraph.GetNode(
		dagger.Path{
			XID:   highestHash,
			XType: BLOCK_TYPE,
		})
	// walk back from the current best block and find the block with requested tx
	// ignore other paths in tree
	obc.BlockGraph.ReverseDFS(CONNECTION_TYPE, highestNode.Path, func(node dagger.Node) bool {
		blk := node.Attributes["block"].(*p2p_pb.BcBlock)
		if block == nil {
			for _, tx := range blk.GetTxs() {
				if tx.GetHash() == txHash {
					block = blk
				}
			}
		}
		return true
	})
	obc.GraphMu.Unlock()
	if block == nil {
		block, err = obc.DB.GetBlockByTx(txHashBytes)
	}
	return block, err
}

func (obc *OverlineBlockchain) GetHighestBlock() (*p2p_pb.BcBlock, error) {
	var block *p2p_pb.BcBlock = nil
	var err error = nil

	obc.Mu.Lock()
	if obc.currentHighestBlock != nil {
		block = obc.currentHighestBlock
		obc.Mu.Unlock()
		return block, nil
	}
	obc.Mu.Unlock()

	block = obc.DB.HighestBlock()
	if block == nil {
		err = errors.New("No highest block defined yet!")
	}

	return block, err
}

func (obc *OverlineBlockchain) SyncProgress() ProgressInfo {
	isSyncing := obc.DB.IsInitialBlockDownload()

	var currentBlock *p2p_pb.BcBlock = nil

	if isSyncing {
		currentBlock = obc.DB.HighestBlock()
	} else {
		obc.Mu.Lock()
		currentBlock = obc.currentHighestBlock
		obc.Mu.Unlock()
	}

	return ProgressInfo{
		Done:                   !isSyncing,
		StartingBlock:          obc.DB.GetSyncStartingBlock(),
		CurrentBlock:           currentBlock,
		HighestPeerBlockHeight: uint64(0),
	}
}
