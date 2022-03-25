package blockchain

import (
	"github.com/autom8ter/dagger"
	"github.com/overline-mining/gool/src/common"
	"github.com/overline-mining/gool/src/database"
	//"github.com/overline-mining/gool/src/validation"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"go.uber.org/zap"
)

const (
	BLOCK_TYPE      = "block"
	CONNECTION_TYPE = "connects"
)

type OverlineBlockchain struct {
	BlockGraph *dagger.Graph
	DB         *database.OverlineDB
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
			"got found previous hash %v -> %v %v",
			block.GetPreviousHash(),
			parentNode.Attributes["block"].(*p2p_pb.BcBlock).GetHash(),
			found,
		)
		_, err := obc.BlockGraph.SetEdge(blkNode.Path, parentNode.Path, dagger.Node{
			Path: dagger.Path{
				XType: CONNECTION_TYPE,
			},
			Attributes: map[string]interface{}{},
		})
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
	}
	obc.BlockGraph.DFS(CONNECTION_TYPE, blkNode.Path, func(node dagger.Node) bool {
		zap.S().Debugf("(%v) DFS: %s", node.Attributes["block"].(*p2p_pb.BcBlock).GetHeight(), node.Attributes["block"].(*p2p_pb.BcBlock).GetHash())
		return true
	})
	zap.S().Debugf("(%v) DFS-ROOT: %s", block.GetHeight(), block.GetHash())
}

func (obc *OverlineBlockchain) AddBlockRange(blocks *p2p_pb.BcBlocks) {
	for _, block := range blocks.Blocks {
		obc.AddBlock(block)
	}
}
