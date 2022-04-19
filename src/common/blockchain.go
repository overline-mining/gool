package common

import (
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"math/big"
)

func BlockOrderingRule(iBlock, jBlock *p2p_pb.BcBlock) bool {
	if iBlock.GetHeight() == jBlock.GetHeight() {
		iDist, _ := new(big.Int).SetString(iBlock.GetTotalDistance(), 10)
		jDist, _ := new(big.Int).SetString(jBlock.GetTotalDistance(), 10)
		compare := iDist.Cmp(jDist)
		if compare == 0 {
			return jBlock.GetTimestamp() < iBlock.GetTimestamp()
		}
		return compare < 0
	}
	return iBlock.GetHeight() < jBlock.GetHeight()
}
