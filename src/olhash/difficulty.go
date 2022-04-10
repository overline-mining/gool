package olhash

import (
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"math/big"
)

// These functions are filled with strange apocrypha from
// the js implementation of the node. Lots of patches.

const (
	MIN_DIFFICULTY     = uint64(200012262029662)
	MIN_DIFFICULTY_AT  = uint64(306612262029662)
	DIFF_BUILD_GENESIS = uint64(290112262029012)
	DIFF_BT_A          = uint64(298678954710059)
	DIFF_BT_X          = uint64(291678954710059)
	DIFF_AT_A          = uint64(311678954710059)
	// BT PERIOD
	BT_A = uint64(1587132412)
	BT_B = uint64(1590157894)
	BT_C = uint64(1590283479)
	BT_D = uint64(1590337524)
	BT_E = uint64(1591666381)
	BT_F = uint64(1596864102)
	BT_G = uint64(1597558249)
	BT_H = uint64(1591666380)
	// AT PERIOD
	AT_A = uint64(1625431234)
	AT_B = uint64(1640266692)
	AT_C = uint64(1642981050)
	AT_D = uint64(1644536250)
	// WIRELESS TRANSMITION MINIMUM
	WIREMIN_A = uint64(1647116144)
	WIREMIN_B = uint64(1647126841)
	// Special block times (magic numbers in js node)
	SPECIAL_BLOCKTIME_ONE   = uint64(1587132449)
	SPECIAL_BLOCKTIME_TWO   = uint64(1590132950)
	SPECIAL_BLOCKTIME_THREE = uint64(1646773191)
	SPECIAL_BLOCKTIME_FOUR  = uint64(1647022731)
	SPECIAL_BLOCKTIME_FIVE  = uint64(1647039583)
)

var BIG_ZERO = new(big.Int).SetInt64(0)
var BIG_ONE = new(big.Int).SetInt64(1)
var BIG_TWO = new(big.Int).SetInt64(2)
var BIG_THREE = new(big.Int).SetInt64(3)
var BIG_THIRTYSIX = new(big.Int).SetInt64(36)
var BIG_EIGHTYSIX = new(big.Int).SetInt64(86)
var BIG_MINUS99 = new(big.Int).SetInt64(-99)
var BIG_TIMEWINDOW = new(big.Int).SetInt64(8)
var BIG_STALEWEIGHT = new(big.Int).SetInt64(10)
var BIG_1000 = new(big.Int).SetInt64(1000)
var BIG_6000 = new(big.Int).SetInt64(6000)
var BIG_MAGICNUMBER1 = new(big.Int).SetInt64(1615500)
var BIG_MAGICNUMBER2 = new(big.Int).SetInt64(161550000)
var BIG_MAGICNUMBER3 = new(big.Int).SetInt64(1215500)
var BIG_MAGICNUMBER4 = new(big.Int).SetInt64(1506600)
var BIG_MAGICNUMBER5 = new(big.Int).SetInt64(16155)
var BIG_MAGICNUMBER6 = new(big.Int).SetInt64(16055)

// why is all of this in BigInt if they squish it back to a uint64 in js?
func GetDifficultyPreExp(currentTimestamp, lastBlockTimestamp uint64, lastBlockDiff, minimumDiff *big.Int, nNewBlocks int64, newestHeaderInBlock *p2p_pb.BlockchainHeader) *big.Int {
	result := new(big.Int)

	actualMinDiff := new(big.Int).SetUint64(MIN_DIFFICULTY)
	if lastBlockTimestamp > WIREMIN_A {
		actualMinDiff.SetUint64(MIN_DIFFICULTY_AT)
	}
	if minimumDiff.Cmp(actualMinDiff) == 1 {
		actualMinDiff.Set(minimumDiff)
	}
	indexedHeaderTime := new(big.Int).SetUint64(newestHeaderInBlock.GetTimestamp())
	indexedHeaderTime.Div(indexedHeaderTime, BIG_1000)
	bigCurrentTime := new(big.Int).SetUint64(currentTimestamp)
	bigLastTime := new(big.Int).SetUint64(lastBlockTimestamp)
	bigNNewBlocks := new(big.Int).SetInt64(nNewBlocks)
	localLastDiff := new(big.Int).Set(lastBlockDiff)

	if indexedHeaderTime.Cmp(bigLastTime) == -1 {
		indexedHeaderTime.Set(bigLastTime)
	}

	timeBound := new(big.Int).Set(indexedHeaderTime)
	timeBound.Add(timeBound, new(big.Int).Mul(BIG_TWO, BIG_TIMEWINDOW))

	elapsedTime := new(big.Int).Sub(bigCurrentTime, bigLastTime)

	if false && elapsedTime.Cmp(BIG_6000) == 1 {
		elapsedTime.Set(BIG_THREE)
		localLastDiff.SetUint64(DIFF_BUILD_GENESIS)
	}

	if lastBlockTimestamp == BT_A || lastBlockTimestamp == SPECIAL_BLOCKTIME_ONE {
		return result.SetUint64(DIFF_BT_A)
	}
	if lastBlockTimestamp == BT_B || lastBlockTimestamp == SPECIAL_BLOCKTIME_TWO {
		return result.SetUint64(DIFF_BT_X)
	}
	if lastBlockTimestamp == BT_C {
		return result.SetUint64(DIFF_BT_X)
	}
	if lastBlockTimestamp == BT_D {
		return result.SetUint64(DIFF_BT_X)
	}
	if lastBlockTimestamp == BT_E {
		return result.SetUint64(DIFF_BT_X)
	}
	if lastBlockTimestamp == BT_F {
		return result.SetUint64(DIFF_BT_X)
	}
	if lastBlockTimestamp == BT_G {
		return result.SetUint64(DIFF_BT_X)
	}
	if lastBlockTimestamp == AT_A {
		return result.SetUint64(DIFF_AT_A)
	}

	staleCost := new(big.Int).Div(new(big.Int).Sub(bigCurrentTime, bigLastTime), BIG_TIMEWINDOW)

	if staleCost.Cmp(BIG_ZERO) == 1 && lastBlockTimestamp <= BT_H {
		elapsedTime.Sub(elapsedTime, staleCost)
	}

	// someone didn't know how to use ||
	if lastBlockTimestamp > AT_B && elapsedTime.Cmp(BIG_THIRTYSIX) == 1 && bigNNewBlocks.Cmp(BIG_ONE) == 1 {
		if (lastBlockTimestamp > AT_D && elapsedTime.Cmp(BIG_EIGHTYSIX) == 1 && bigNNewBlocks.Cmp(BIG_ONE) == 1) ||
			(lastBlockTimestamp < AT_C) {
			elapsedTime.Mul(elapsedTime, bigNNewBlocks)
		}
	}

	w := new(big.Int).Set(BIG_ZERO)
	elapsedTimeBonus := new(big.Int).Add(elapsedTime, new(big.Int).Add(new(big.Int).Sub(elapsedTime, BIG_STALEWEIGHT), bigNNewBlocks))

	if lastBlockTimestamp >= SPECIAL_BLOCKTIME_THREE && elapsedTime.Cmp(BIG_EIGHTYSIX) == 1 {
		w.Set(elapsedTimeBonus) // original elapsed time bonus
		elapsedTimeBonus.Mul(elapsedTime, bigNNewBlocks)
	}

	elapsedTime.Set(elapsedTimeBonus)

	x := new(big.Int).Sub(BIG_ONE, new(big.Int).Div(elapsedTime, BIG_TIMEWINDOW))
	y := new(big.Int)
	n := new(big.Int)
	z := new(big.Int).SetInt64(0)
	m := new(big.Int).SetInt64(0)

	if x.Cmp(BIG_MINUS99) == -1 {
		z.Set(x)
		if lastBlockTimestamp < 1647022731 {
			x.Set(BIG_MINUS99)
		}
		if lastBlockTimestamp > WIREMIN_A {
			x.Set(BIG_MINUS99)
		}
	}

	if lastBlockTimestamp > AT_D {

		if x.Cmp(BIG_ZERO) == -1 {
			if lastBlockTimestamp >= WIREMIN_A {
				n.Sub(BIG_MAGICNUMBER1, elapsedTimeBonus) // BigInt(1615500) - elapsedTimeBonus; // <-- AT
			} else if lastBlockTimestamp >= SPECIAL_BLOCKTIME_FOUR {
				if lastBlockTimestamp >= SPECIAL_BLOCKTIME_FIVE {
					n.Sub(BIG_MAGICNUMBER2, elapsedTimeBonus) // BigInt(161550000) - elapsedTimeBonus; // <-- AT
				} else {
					n.Sub(BIG_MAGICNUMBER1, elapsedTimeBonus) //BigInt(1615500) - elapsedTimeBonus; // <-- AT
				}
				if n.Cmp(BIG_ZERO) == -1 {
					n.Set(BIG_ONE) // BigInt(1);
				}
			} else {
				n.Add(BIG_MAGICNUMBER1, elapsedTimeBonus) // BigInt(1615500) + elapsedTimeBonus; // <-- AT2
			}
		} else {
			if lastBlockTimestamp >= WIREMIN_A {
				n.Add(BIG_MAGICNUMBER3, elapsedTimeBonus) // = BigInt(1215500) - elapsedTimeBonus; // <-- AT
			} else if lastBlockTimestamp >= SPECIAL_BLOCKTIME_FIVE {
				n.Sub(BIG_MAGICNUMBER2, elapsedTimeBonus) //BigInt(161550000) - elapsedTimeBonus; // <-- AT
			} else {
				n.Sub(BIG_MAGICNUMBER4, elapsedTimeBonus) // BigInt(1506600) + elapsedTimeBonus; // <-- AT2
			}
		}
	} else {
		// y = bigPreviousDifficulty -> OVERLINE: 10062600 // AT: 1615520 // BT: ((32 * 16) / 2PI ) * 10 = 815 chain count + hidden chain = 508
		if x.Cmp(BIG_ZERO) == -1 {
			// n = BigInt(1615520) + elapsedTimeBonus // <-- BT
			n.Add(BIG_MAGICNUMBER5, elapsedTimeBonus) //  = BigInt(16155) + elapsedTimeBonus; // <-- AT
		} else {
			// n = BigInt(1506600) + elapsedTimeBonus <-- BT
			n.Add(BIG_MAGICNUMBER6, elapsedTimeBonus) // BigInt(15066) + elapsedTimeBonus; // <-- AT
		}
	}

	y.Div(localLastDiff, bigNNewBlocks)
	// x = x * y
	x.Mul(x, y)
	m.Set(x)
	if lastBlockTimestamp < SPECIAL_BLOCKTIME_FOUR {
		x.Add(new(big.Int).Add(x, localLastDiff), elapsedTimeBonus)
	} else {
		x.Add(x, localLastDiff)
	}

	// x < ensure follows wireless minimum
	// prevent web3 routers from competing with GPU miners
	if x.Cmp(actualMinDiff) == -1 && lastBlockTimestamp > WIREMIN_B {
		return result.SetUint64(MIN_DIFFICULTY_AT)
	}

	if x.Cmp(new(big.Int).SetUint64(MIN_DIFFICULTY)) == -1 {
		return result.SetUint64(MIN_DIFFICULTY)
	}

	// x < minimalDifficulty
	if x.Cmp(actualMinDiff) == -1 {
		return actualMinDiff
	}

	return result.Set(x)
}
