package blockchain

import (
	"github.com/autom8ter/dagger"
	"github.com/overline-mining/gool/src/common"
	"github.com/overline-mining/gool/src/database"
	//"github.com/overline-mining/gool/src/validation"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"go.uber.org/zap"
	"sync"
)

const (
	BLOCK_TYPE      = "block"
	CONNECTION_TYPE = "connects"
)

type OverlineBlockchain struct {
	Mu                               sync.Mutex
	followMu                         sync.Mutex
	Heads                            map[string]bool
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
		zap.S().Infof(
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
	// first clean up the chain state, connect heads that may have arrived out of order
	for h, _ := range heads {
		headBlock, found := obc.BlockGraph.GetNode(
			dagger.Path{
				XID:   h,
				XType: BLOCK_TYPE,
			})
		if !found { // heads must appear in the block graph!
			zap.S().Panicf("Could not find head block: %v", common.BriefHash(h))
		}
		// get the head's parent node if it exists
		parentNode, found := obc.BlockGraph.GetNode(
			dagger.Path{
				XID:   headBlock.Attributes["block"].(*p2p_pb.BcBlock).GetPreviousHash(),
				XType: BLOCK_TYPE,
			})
		if found {
			zap.S().Debugf(
				"found previous hash in chain graph for head %v -> %v %v",
				common.BriefHash(block.GetPreviousHash()),
				common.BriefHash(parentNode.Attributes["block"].(*p2p_pb.BcBlock).GetHash()),
				found,
			)
			err := obc.connectToChildBlock(parentNode, headBlock)
			if err != nil {
				zap.S().Panic(err)
			}
			// remove the now-integrated head from the list of heads
			delete(heads, h)
		}
	}
	// copy the updated heads list back to the lock-protected version
	obc.Mu.Lock()
	obc.Heads = make(map[string]bool)
	for h, v := range heads {
		obc.Heads[h] = v
	}
	obc.Mu.Unlock()
	// display all heads, mark longest chain, pop to db ingestion if connected and depth > maturity depth
	for hash, _ := range heads {
		headNode, found := obc.BlockGraph.GetNode(
			dagger.Path{
				XID:   hash,
				XType: BLOCK_TYPE,
			})
		if !found {
			zap.S().Panicf("Could not find head node %v", common.BriefHash(hash))
		}
		obc.BlockGraph.DFS(CONNECTION_TYPE, headNode.Path, func(node dagger.Node) bool {
			blk := node.Attributes["block"].(*p2p_pb.BcBlock)
			zap.S().Debugf("(%v) DFS: %s -> %s", blk.GetHeight(), blk.GetHash(), blk.GetPreviousHash())
			return true
		})
		headBlock := headNode.Attributes["block"].(*p2p_pb.BcBlock)
		zap.S().Debugf("(%v) DFS-ROOT: %s -> %s", headBlock.GetHeight(), headBlock.GetHash(), headBlock.GetPreviousHash())
	}
}

func (obc *OverlineBlockchain) AddBlockRange(blocks *p2p_pb.BcBlocks) {
	for _, block := range blocks.Blocks {
		obc.AddBlock(block)
	}
}
