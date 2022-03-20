package validation

import (
	p2p_pb "github.com/overline-mining/gool/src/protos"
	//"go.uber.org/zap"
)

func ValidateBlockRange(startingBlock *p2p_pb.BcBlock, blocks []*p2p_pb.BcBlock) (bool, int) {
	n := len(blocks)
	for i := n - 1; i > 0; i-- {
		if !OrderedBlockPairIsValid(blocks[i-1], blocks[i]) {
			return false, i
		}
	}
	if !OrderedBlockPairIsValid(startingBlock, blocks[0]) {
		return false, 0
	}
	return true, -1
}

func OrderedBlockPairIsValid(low, high *p2p_pb.BcBlock) bool {
	return ((high.GetHeight()-1 == low.GetHeight()) && (high.GetPreviousHash() == low.GetHash()))
}
