package validation

import (
	"github.com/overline-mining/gool/src/common"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"go.uber.org/zap"
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

func globalContiguityProblems(low, high *p2p_pb.BcBlock) bool {
	return ((low.GetHeight() == uint64(641452) && high.GetHeight() == uint64(641453)) || // stuck chain catch up
		(low.GetHeight() == uint64(1771220) && high.GetHeight() == uint64(1771221)) || // stuck chain catch up
		(low.GetHeight() == uint64(2921635) && high.GetHeight() == uint64(2921636))) // multiple rovers out of order
}

func btcContiguityProblems(low, high *p2p_pb.BcBlock) bool {
	return false
}

func ethContiguityProblems(low, high *p2p_pb.BcBlock) bool {
	return ((low.GetHeight() == uint64(6825682) || high.GetHeight() == uint64(6825682)) || // 6825682 eth is missing blocks 14426493 - 14426497
		(low.GetHeight() == uint64(6823254) || high.GetHeight() == uint64(6823254)) || // 6823254 eth is missing blocks 14425278 - 14425279
		(low.GetHeight() == uint64(6820589) || high.GetHeight() == uint64(6820589)) || // 6820589 eth is missing blocks 14423772 - 14423775
		(low.GetHeight() == uint64(6817490) || high.GetHeight() == uint64(6817490)) || // 6817490 eth is missing block 14422525
		(low.GetHeight() == uint64(6809276) || high.GetHeight() == uint64(6809276)) || // 6809276 eth is missing blocks 14419404 - 14419405
		(low.GetHeight() == uint64(6778570) || high.GetHeight() == uint64(6778570)) || // 6778570 eth is missing blocks 14407958 - 14407966
		(low.GetHeight() == uint64(6776209) || high.GetHeight() == uint64(6776209)) || // 6776209 eth is missing blocks 14406915 - 14406917
		(low.GetHeight() == uint64(6772753) || high.GetHeight() == uint64(6772753)) || // 6772753 eth is missing blocks 14404645 - 14404648
		(low.GetHeight() == uint64(6761621) || high.GetHeight() == uint64(6761621)) || // 6761621 eth is missing blocks 14398965 - 14398968
		(low.GetHeight() == uint64(6754138) || high.GetHeight() == uint64(6754138)) || // 6754138 eth is missing block 14395958
		(low.GetHeight() == uint64(6682094) || high.GetHeight() == uint64(6682094)) || // 6682094 eth is missing blocks 14365666 - 14365671
		(low.GetHeight() == uint64(6657585) && high.GetHeight() == uint64(6657586)) || // 6657585 eth does not increment to 6657586
		(low.GetHeight() == uint64(6638444) && high.GetHeight() == uint64(6638445)) || // 6638444 eth does not increment to 6638445
		(low.GetHeight() == uint64(6627338) && high.GetHeight() == uint64(6627339)) || // 6627338 eth does not increment to 6627339
		(low.GetHeight() == uint64(6620449) && high.GetHeight() == uint64(6620450)) || // 6620449 does not increment to 6620450
		(low.GetHeight() == uint64(6620262) && high.GetHeight() == uint64(6620263)) || // 6620262 does not increment to 6620263
		(low.GetHeight() == uint64(6611014) && high.GetHeight() == uint64(6611015)) || // 6611014 eth does not increment to 6611015
		(low.GetHeight() == uint64(6539050) && high.GetHeight() == uint64(6539051)) || // 6539050 does not increment to 6539051
		(low.GetHeight() == uint64(6457430) && high.GetHeight() == uint64(6457431)) || // 6457430 does not increment to 6457431
		(low.GetHeight() == uint64(6453532) && high.GetHeight() == uint64(6453533)) || // 6453532 eth does not point to 6453533 hash
		(low.GetHeight() == uint64(6392780) && high.GetHeight() == uint64(6392781)) || // 6392780 eth does not increment to 6392781
		(low.GetHeight() == uint64(6389354) && high.GetHeight() == uint64(6389355)) || // 6389354 eth does not increment to 6389355
		(low.GetHeight() == uint64(6075150) && high.GetHeight() == uint64(6075151)) || // 6075150 eth is not hash pointed to by 6075151
		(low.GetHeight() == uint64(6072723) && high.GetHeight() == uint64(6072724)) || // 6072723 eth does not increment to 6072724
		(low.GetHeight() == uint64(6061792) && high.GetHeight() == uint64(6061793)) || // 6061792 eth is not hash pointed to by 6061793
		(low.GetHeight() == uint64(6061585) && high.GetHeight() == uint64(6061586)) || // 6061585 eth is not hash pointed to by 6061586
		(low.GetHeight() == uint64(6058196) && high.GetHeight() == uint64(6058197)) || // 6058196 eth is not hash pointed to by 6058197
		(low.GetHeight() == uint64(6013776) && high.GetHeight() == uint64(6013777)) || // 6013776 eth does not increment to 6013777
		(low.GetHeight() == uint64(6005529) && high.GetHeight() == uint64(6005530)) || // 6005529 eth does not increment to 6005530
		(low.GetHeight() == uint64(5944824) && high.GetHeight() == uint64(5944825)) || // 5944824 eth is not hash pointed to by 5944825
		(low.GetHeight() == uint64(5943292) && high.GetHeight() == uint64(5943293)) || // 5943292 eth does not increment to 5943293
		(low.GetHeight() == uint64(5943089) && high.GetHeight() == uint64(5943090)) || // 5943089 eth is not hash pointed to by 5943090
		(low.GetHeight() == uint64(5943048) && high.GetHeight() == uint64(5943049)) || // 5943048 eth is not hash pointed to by 5943049
		(low.GetHeight() == uint64(5942422) && high.GetHeight() == uint64(5942423)) || // 5942422 eth does not increment to 5942423
		(low.GetHeight() == uint64(5761152) && high.GetHeight() == uint64(5761153)) || // 5761152 eth does not increment to 5761153
		(low.GetHeight() == uint64(5124029) && high.GetHeight() == uint64(5124030)) || // 5124029 eth does not increment to 5124030
		(low.GetHeight() == uint64(5124024) && high.GetHeight() == uint64(5124025)) || // 5124024 eth does not increment to 5124025
		(low.GetHeight() == uint64(5124020) && high.GetHeight() == uint64(5124021)) || // 5124020 eth does not increment to 5124021
		(low.GetHeight() == uint64(5123986) && high.GetHeight() == uint64(5123987)) || // 5123986 eth does not increment to 5123987
		(low.GetHeight() == uint64(5123968) && high.GetHeight() == uint64(5123969)) || // 5123968 eth does not increment to 5123969
		(low.GetHeight() == uint64(5123963) && high.GetHeight() == uint64(5123964)) || // 5123963 eth does not increment to 5123964
		(low.GetHeight() == uint64(3378927) || high.GetHeight() == uint64(3378927)) || // 3378927 eth missing 12206231
		(low.GetHeight() == uint64(3121968) && high.GetHeight() == uint64(3121969)) || // 3121968 eth does not increment to 3121969
		(low.GetHeight() == uint64(2820590) && high.GetHeight() == uint64(2820591))) // 2820590 eth does not increment to 2820591
}

func lskContiguityProblems(low, high *p2p_pb.BcBlock) bool {
	return ((low.GetHeight() == uint64(6826628) || high.GetHeight() == uint64(6826628)) || // 6826628 lsk is missing blocks 18052855 - 18052863
		(low.GetHeight() == uint64(6825682) || high.GetHeight() == uint64(6825682)) || // 6825682 lsk is missing blocks 18052147 - 18052155
		(low.GetHeight() == uint64(6610732) && high.GetHeight() == uint64(6610733)) || // 6610732 lsk does not increment to 6610733
		(low.GetHeight() == uint64(6249671) && high.GetHeight() == uint64(6249672)) || // 6249671 lsk does not increment to 6249672
		(low.GetHeight() == uint64(5958529) && high.GetHeight() == uint64(5958530)) || // 5958529 lsk does not increment to 5958530
		(low.GetHeight() == uint64(5957773) && high.GetHeight() == uint64(5957774)) || // 5957773 lsk does not increment to 5957774
		(low.GetHeight() == uint64(5950202) && high.GetHeight() == uint64(5950203)) || // 5950202 lsk does not increment to 5950203
		(low.GetHeight() == uint64(5945645) && high.GetHeight() == uint64(5945646)) || // 5945645 lsk does not increment to 5945646
		(low.GetHeight() == uint64(5741082) && high.GetHeight() == uint64(5741083)) || // 5741082 lsk does not increment to 5741083
		(low.GetHeight() == uint64(5032519) && high.GetHeight() == uint64(5032520)) || // 5032519 lsk does not increment to 5032520
		(low.GetHeight() == uint64(4860129) && high.GetHeight() == uint64(4860130)) || // 4860129 lsk does not increment to 4860130
		(low.GetHeight() == uint64(4851654) && high.GetHeight() == uint64(4851655)) || // 4851654 lsk does not increment to 4851655
		(low.GetHeight() == uint64(2927292) && high.GetHeight() == uint64(2927293)) || // 2927292 lsk does not increment to 2927293
		(low.GetHeight() == uint64(2921767) && high.GetHeight() == uint64(2921768)) || // 2921767 lsk does not increment to 2921768
		(low.GetHeight() == uint64(2877093) && high.GetHeight() == uint64(2877094))) // 2877093 lsk does not increment to 2877094
}

func neoContiguityProblems(low, high *p2p_pb.BcBlock) bool {
	return ((low.GetHeight() == uint64(6686205) || high.GetHeight() == uint64(6686205)) || // 6686205 neo is missing block 8900857
		(low.GetHeight() == uint64(5937782) && high.GetHeight() == uint64(5937783)) || // 5937782 neo does not increment to 5937783
		(low.GetHeight() == uint64(5936375) && high.GetHeight() == uint64(5936376)) || // 5936375 neo does not increment to 5936376
		(low.GetHeight() == uint64(2927292) && high.GetHeight() == uint64(2927293)) || // 2927292 neo does not increment to 2927293
		(low.GetHeight() == uint64(2921767) && high.GetHeight() == uint64(2921768))) // 2921767 neo does not increment to 2921768
}

func wavContiguityProblems(low, high *p2p_pb.BcBlock) bool {
	return ((low.GetHeight() == uint64(6823182) || high.GetHeight() == uint64(6823182)) || // 6823182 wav is missing block 3037758
		(low.GetHeight() == uint64(6776437) || high.GetHeight() == uint64(6776437)) || // 6776437 wav is missing blocks 3033379 - 3033382
		(low.GetHeight() == uint64(6644362) && high.GetHeight() == uint64(6644363)) || // 6644362 wav does not increment to 6644363
		(low.GetHeight() == uint64(6641219) && high.GetHeight() == uint64(6641220)) || // 6641219 wav does not increment to 6641220
		(low.GetHeight() == uint64(6633144) && high.GetHeight() == uint64(6633145)) || // 6633144 wav does not increment to 6633145
		(low.GetHeight() == uint64(6625383) && high.GetHeight() == uint64(6625384)) || // 6625383 wav does not increment to 6625384
		(low.GetHeight() == uint64(6621066) && high.GetHeight() == uint64(6621067)) || // 6621066 wav does not increment to 6621067
		(low.GetHeight() == uint64(6616183) && high.GetHeight() == uint64(6616184)) || // 6616183 wav does not increment to 6616184
		(low.GetHeight() == uint64(6571760) && high.GetHeight() == uint64(6571761)) || // 6571760 wav does not increment to 6571761
		(low.GetHeight() == uint64(6463192) && high.GetHeight() == uint64(6463193)) || // 6463192 wav does not increment to 6463193
		(low.GetHeight() == uint64(6448350) && high.GetHeight() == uint64(6448351)) || // 6448350 wav does not increment to 6448351
		(low.GetHeight() == uint64(6229663) && high.GetHeight() == uint64(6229664)) || // 6229663 wav does not increment to 6229664
		(low.GetHeight() == uint64(6133666) && high.GetHeight() == uint64(6133667)) || // 6133666 wav does not increment to 6133667
		(low.GetHeight() == uint64(6078335) && high.GetHeight() == uint64(6078336)) || // 6078335 wav does not increment to 6078336
		(low.GetHeight() == uint64(5948727) && high.GetHeight() == uint64(5948728)) || // 5948727 wav does not increment to 5948728
		(low.GetHeight() == uint64(5944634) && high.GetHeight() == uint64(5944635)) || // 5944634 wav does not increment to 5944635
		(low.GetHeight() == uint64(5908377) && high.GetHeight() == uint64(5908378)) || // 5908377 wav does not increment to 5908378
		(low.GetHeight() == uint64(5890170) && high.GetHeight() == uint64(5890171)) || // 5890170 wav does not increment to 5890171
		(low.GetHeight() == uint64(5890169) && high.GetHeight() == uint64(5890170)) || // 5890169 wav does not increment to 5890170
		(low.GetHeight() == uint64(5870867) && high.GetHeight() == uint64(5870868)) || // 5870867 wav does not increment to 5870868
		(low.GetHeight() == uint64(5792335) && high.GetHeight() == uint64(5792336)) || // 5792335 wav does not increment to 5792336
		(low.GetHeight() == uint64(5760218) && high.GetHeight() == uint64(5760219))) // 5760218 wav does not increment to 5760219
}

func OrderedBlockPairIsValid(low, high *p2p_pb.BcBlock) bool {
	return orderedBlockPairIsValid(low, high, false)
}

func OrderedBlockPairIsValidStrict(low, high *p2p_pb.BcBlock) bool {
	return orderedBlockPairIsValid(low, high, true)
}

func orderedBlockPairIsValid(low, high *p2p_pb.BcBlock, isStrict bool) bool {
	const nitPickedValidationHeight = uint64(6850000)
	if (high.GetHeight()-1 != low.GetHeight()) || (high.GetPreviousHash() != low.GetHash()) {
		return false
	}
	if low.GetHeight() != 1 { // do not validate headers if comparing to genesis block
		if !globalContiguityProblems(low, high) {
			if !btcContiguityProblems(low, high) && !HeaderRangeIsContiguous(low.GetBlockchainHeaders().GetBtc(), high.GetBlockchainHeaders().GetBtc()) {
				if low.GetHeight() >= nitPickedValidationHeight && !isStrict {
					zap.S().Warnf("Problem in BTC contiguity spanning blocks %v %v -> %v %v", low.GetHeight(), common.BriefHash(low.GetHash()), high.GetHeight(), common.BriefHash(high.GetHash()))
				} else {
					return false
				}
			}
			if !ethContiguityProblems(low, high) && !HeaderRangeIsContiguous(low.GetBlockchainHeaders().GetEth(), high.GetBlockchainHeaders().GetEth()) {
				if low.GetHeight() >= nitPickedValidationHeight && !isStrict {
					zap.S().Warnf("Problem in ETH contiguity spanning blocks %v %v -> %v %v", low.GetHeight(), common.BriefHash(low.GetHash()), high.GetHeight(), common.BriefHash(high.GetHash()))
				} else {
					return false
				}
			}
			if !lskContiguityProblems(low, high) && !HeaderRangeIsContiguous(low.GetBlockchainHeaders().GetLsk(), high.GetBlockchainHeaders().GetLsk()) {
				if low.GetHeight() >= nitPickedValidationHeight && !isStrict {
					zap.S().Warnf("Problem in LSK contiguity spanning blocks %v %v -> %v %v", low.GetHeight(), common.BriefHash(low.GetHash()), high.GetHeight(), common.BriefHash(high.GetHash()))
				} else {
					return false
				}
			}
			if !neoContiguityProblems(low, high) && !HeaderRangeIsContiguous(low.GetBlockchainHeaders().GetNeo(), high.GetBlockchainHeaders().GetNeo()) {
				if low.GetHeight() >= nitPickedValidationHeight && !isStrict {
					zap.S().Warnf("Problem in NEO contiguity spanning blocks %v %v -> %v %v", low.GetHeight(), common.BriefHash(low.GetHash()), high.GetHeight(), common.BriefHash(high.GetHash()))
				} else {
					return false
				}
			}
			if !wavContiguityProblems(low, high) && !HeaderRangeIsContiguous(low.GetBlockchainHeaders().GetWav(), high.GetBlockchainHeaders().GetWav()) {
				if low.GetHeight() >= nitPickedValidationHeight && !isStrict {
					zap.S().Warnf("Problem in WAV contiguity spanning blocks %v %v -> %v %v", low.GetHeight(), common.BriefHash(low.GetHash()), high.GetHeight(), common.BriefHash(high.GetHash()))
				} else {
					return false
				}
			}
		}

	}
	return true
}

func isSameHeightOrChained(low, high *p2p_pb.BlockchainHeader) bool {
	correctOrder := (low.GetHeight() == high.GetHeight() || ((high.GetHeight()-1 == low.GetHeight()) && (high.GetPreviousHash() == low.GetHash())))
	if !correctOrder {
		return (low.GetHeight() == high.GetHeight() || ((low.GetHeight()-1 == high.GetHeight()) && (low.GetPreviousHash() == high.GetHash())))
	}
	return correctOrder
}

func isChained(low, high []*p2p_pb.BlockchainHeader) bool {
	if isSameHeightOrChained(low[len(low)-1], high[0]) {
		return true
	}
	if low[0].GetHeight() <= high[0].GetHeight() && low[len(low)-1].GetHeight() <= high[len(high)-1].GetHeight() {
		return true
	}
	if high[0].GetHeight() <= low[0].GetHeight() && high[len(high)-1].GetHeight() <= low[len(low)-1].GetHeight() {
		return true
	}
	if low[0].GetHeight() <= high[0].GetHeight() && low[len(low)-1].GetHeight() >= high[len(high)-1].GetHeight() {
		return true
	}
	if high[0].GetHeight() <= low[0].GetHeight() && high[len(high)-1].GetHeight() >= low[len(low)-1].GetHeight() {
		return true
	}
	return false
}

func HeaderRangeIsContiguous(low, high []*p2p_pb.BlockchainHeader) bool {
	// special case first
	if len(low) == 1 && len(high) == 1 {
		return isSameHeightOrChained(low[0], high[0])
	}
	lowLast := len(low) - 1
	highLast := len(high) - 1
	for i := 0; i < lowLast; i++ {
		if low[i].GetHash() != low[i+1].GetPreviousHash() {
			return false
		}
	}
	for i := 0; i < highLast; i++ {
		if high[i].GetHash() != high[i+1].GetPreviousHash() {
			return false
		}
	}
	return (isChained(low, high))
}
