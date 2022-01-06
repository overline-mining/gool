package validation

import (
	"encoding/hex"
	//"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"

	"github.com/overline-mining/gool/src/common"
	"github.com/overline-mining/gool/src/genesis"
	"github.com/overline-mining/gool/src/olhash"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"go.uber.org/zap"
)

func IsValidBlock(block *p2p_pb.BcBlock) (bool, error) {
	if block == nil {
		return false, errors.New("Block presented for validation is nil!")
	}

	// pass over stuck blocks or phases with no validation (!?)
	// not this node's fault
	if isSkippableBlock(block) {
		return true, nil
	}

	if !TheBlockChainFingerprintMatchGenesisBlock(block) {
		errStr := fmt.Sprintf("%v failed: TheBlockChainFingerprintMatchGenesisBlock", common.BriefHash(block.GetHash()))
		zap.S().Errorf(errStr)
		return false, errors.New(errStr)
	}

	if !NumberOfBlockchainsNeededMatchesChildBlock(block) {
		errStr := fmt.Sprintf("%v failed: NumberOfBlockchainsNeededMatchesChildBlock", common.BriefHash(block.GetHash()))
		zap.S().Errorf(errStr)
		return false, errors.New(errStr)
	}
	if !IfMoreThanOneHeaderPerBlockchainAreTheyOrdered(block) {
		errStr := fmt.Sprintf("%v failed: IfMoreThanOneHeaderPerBlockchainAreTheyOrdered", common.BriefHash(block.GetHash()))
		zap.S().Errorf(errStr)
		return false, errors.New(errStr)
	}
	if !IsChainRootCorrectlyCalculated(block) {
		errStr := fmt.Sprintf("%v failed: IsChainRootCorrectlyCalculated for %d, %s", common.BriefHash(block.GetHash()), block.GetHeight(), block.GetHash())
		zap.S().Errorf(errStr)
		return false, errors.New(errStr)
	}
	if !IsFieldLengthBounded(block) {
		errStr := fmt.Sprintf("%v failed: IsFieldLengthBounded at height %v", common.BriefHash(block.GetHash()), block.GetHeight())
		zap.S().Errorf(errStr)
		return false, errors.New(errStr)
	}

	if block.GetHeight() > 665616 {
		if !IsMerkleRootCorrectlyCalculated(block) {
			errStr := fmt.Sprintf("%v failed: IsMerkleRootCorrectlyCalculated at height %v", common.BriefHash(block.GetHash()), block.GetHeight())
			zap.S().Errorf(errStr)
			return false, errors.New(errStr)
		}
	}

	if !IsDistanceAboveDifficulty(block) {
		errStr := fmt.Sprintf("%v failed: IsDistanceAboveDifficulty at height %v", common.BriefHash(block.GetHash()), block.GetHeight())
		zap.S().Errorf(errStr)
		return false, errors.New(errStr)
	}

	if !IsDistanceCorrectlyCalculated(block) {
		errStr := fmt.Sprintf("%v failed: IsDistanceCorrectlyCalculated at height %v", common.BriefHash(block.GetHash()), block.GetHeight())
		zap.S().Errorf(errStr)
		return false, errors.New(errStr)
	}

	if !IsValidBlockTime(block) {
		errStr := fmt.Sprintf("%v failed: IsValidBlockTime at height %v", common.BriefHash(block.GetHash()), block.GetHeight())
		zap.S().Errorf(errStr)
		return false, errors.New(errStr)
	}

	return true, nil
}

func TheBlockChainFingerprintMatchGenesisBlock(block *p2p_pb.BcBlock) bool {
	return block.GetBlockchainFingerprintsRoot() == genesis.BLOCKCHAIN_FINGERPRINTS_ROOT
}

func NumberOfBlockchainsNeededMatchesChildBlock(block *p2p_pb.BcBlock) bool {
	// genesis block does not have child headers
	if block.GetHash() == genesis.HASH && block.GetHeight() == 1 {
		return true
	}

	headers := block.GetBlockchainHeaders()
	if headers == nil {
		return false
	}

	nFilledHeaders := 0
	if len(headers.GetBtc()) > 0 {
		nFilledHeaders++
	}
	if len(headers.GetEth()) > 0 {
		nFilledHeaders++
	}
	if len(headers.GetLsk()) > 0 {
		nFilledHeaders++
	}
	if len(headers.GetNeo()) > 0 {
		nFilledHeaders++
	}
	if len(headers.GetWav()) > 0 {
		nFilledHeaders++
	}

	return nFilledHeaders == genesis.CHILD_BLOCKCHAIN_COUNT
}

func IfMoreThanOneHeaderPerBlockchainAreTheyOrdered(block *p2p_pb.BcBlock) bool {
	headers := block.GetBlockchainHeaders()

	if len(headers.GetBtc()) > 1 {
		if !headerListIsOrderedByHeight(headers.GetBtc()) {
			return false
		}
	}
	if len(headers.GetEth()) > 1 {
		if !headerListIsOrderedByHeight(headers.GetEth()) {
			return false
		}
	}
	if len(headers.GetLsk()) > 1 {
		if !headerListIsOrderedByHeight(headers.GetLsk()) {
			return false
		}
	}
	if len(headers.GetNeo()) > 1 {
		if !headerListIsOrderedByHeight(headers.GetNeo()) {
			return false
		}
	}
	if len(headers.GetWav()) > 1 {
		if !headerListIsOrderedByHeight(headers.GetWav()) {
			return false
		}
	}

	return true
}

func IsChainRootCorrectlyCalculated(block *p2p_pb.BcBlock) bool {
	const (
		defaultHash1 = "75fa3705a0d87019897583bb3314766abc798915262e096c009f1704fb1f1f41"
		defaultHash2 = "0bb5f5313ea0acb1b116812990bdf3d383b9bbb1e66c562e5af2c1bef15b2f93"
	)

	if block.GetHash() == defaultHash1 || block.GetHash() == defaultHash2 {
		zap.S().Debugf("Skipping validation \"IsChainRootCorrectlyCalculated\" for hash %v", block.GetHash())
		return true
	}

	receivedChainRoot := block.GetChainRoot()
	//zap.S().Debugf("Expected chain root -> %v", receivedChainRoot)
	expectedBlockHashes := getChildrenBlocksHashes(block.GetBlockchainHeaders(), block.GetHeight())
	expectedChainRoot := olhash.Blake2bl(getChildrenRootHash(expectedBlockHashes).String())
	//zap.S().Debugf("Recalculated chain root -> %v", expectedChainRoot)
	return receivedChainRoot == expectedChainRoot
}

func IsFieldLengthBounded(block *p2p_pb.BcBlock) bool {
	return true
}

func IsMerkleRootCorrectlyCalculated(block *p2p_pb.BcBlock) bool {
	expectedMerkle := block.GetMerkleRoot()

	toHash := []string{}
	toHash = append(toHash, getChildrenBlocksHashes(block.GetBlockchainHeaders(), block.GetHeight())...)

	for _, tx := range block.GetTxs() {
		toHash = append(toHash, tx.GetHash())
	}

	toHash = append(toHash, block.GetDifficulty())
	toHash = append(toHash, block.GetMiner())
	toHash = append(toHash, fmt.Sprintf("%d", block.GetHeight()))
	toHash = append(toHash, fmt.Sprintf("%d", block.GetVersion()))
	toHash = append(toHash, fmt.Sprintf("%d", block.GetSchemaVersion()))
	toHash = append(toHash, fmt.Sprintf("%d", block.GetNrgGrant()))
	toHash = append(toHash, genesis.BLOCKCHAIN_FINGERPRINTS_ROOT)

	merkleRoot := olhash.Blake2bl(toHash[0])
	for _, h := range toHash[1:] {
		merkleRoot = olhash.Blake2bl(merkleRoot + h)
	}

	//zap.S().Debugf("Calculated MerkleRoot %v == %v (expected)", merkleRoot, expectedMerkle)

	return expectedMerkle == merkleRoot
}

func IsDistanceAboveDifficulty(block *p2p_pb.BcBlock) bool {
	receivedDistance, success := new(big.Int).SetString(block.GetDistance(), 10)
	if receivedDistance == nil {
		return success
	}
	receivedDifficulty, success := new(big.Int).SetString(block.GetDifficulty(), 10)
	if receivedDifficulty == nil {
		return success
	}
	return (receivedDistance.Cmp(receivedDifficulty) == 1)
}

func IsDistanceCorrectlyCalculated(block *p2p_pb.BcBlock) bool {
	// we have to use a tolerance here because of javascript's weird floating point
	// implementation
	const tolValue = uint64(10)
	tol := new(big.Int).SetUint64(tolValue)
	work := prepareWork(block)
	reCalcDistance := olhash.EvalString(work, block.GetMiner(), block.GetMerkleRoot(), block.GetNonce(), block.GetTimestamp())
	reCalcDistanceBN := new(big.Int).SetUint64(reCalcDistance)
	distance, success := new(big.Int).SetString(block.GetDistance(), 10)
	if !success {
		return false
	}
	diff := new(big.Int).Sub(reCalcDistanceBN, distance)
	diff.Abs(diff)
	if diff.Uint64() != 0 {
		zap.S().Warnf("Distances different but within tolerance: %v != %v", reCalcDistanceBN, distance)
	}
	return diff.Cmp(tol) <= 0
}

func IsValidBlockTime(block *p2p_pb.BcBlock) bool {
	const (
		timeWindowVal = uint64(2855000)
		timeValHeight = uint64(5900000)
	)

	if block.GetHeight() < timeValHeight {
		return true
	}

	timeWindow := new(big.Int).SetUint64(timeWindowVal)

	timestamp := new(big.Int).SetUint64(block.GetTimestamp())
	timestamp.Mul(timestamp, new(big.Int).SetUint64(1000))
	sortedHeaders := timeSortedBlockHeaders(block.GetBlockchainHeaders())
	newest := new(big.Int).SetUint64(sortedHeaders[0].GetTimestamp())
	oldest := new(big.Int).SetUint64(sortedHeaders[len(sortedHeaders)-1].GetTimestamp())

	if new(big.Int).Add(newest, timeWindow).Cmp(timestamp) == -1 {
		if new(big.Int).Add(oldest, timeWindow).Cmp(timestamp) == 1 {
			return true
		}
		return false
	}
	return true
}

func timeSortedBlockHeaders(headers *p2p_pb.BlockchainHeaders) []*p2p_pb.BlockchainHeader {
	out := make([]*p2p_pb.BlockchainHeader, 0)
	out = append(out, headers.GetBtc()...)
	out = append(out, headers.GetEth()...)
	out = append(out, headers.GetLsk()...)
	out = append(out, headers.GetNeo()...)
	out = append(out, headers.GetWav()...)

	sort.Slice(out, func(i, j int) bool {
		if out[j].GetTimestamp() == out[i].GetTimestamp() {
			return out[j].GetHeight() < out[i].GetHeight()
		}
		return out[j].GetTimestamp() < out[i].GetTimestamp()
	})

	return out
}

func prepareWork(block *p2p_pb.BcBlock) string {
	prevHashBytes, err := hex.DecodeString(block.GetPreviousHash())
	if err != nil {
		zap.S().Fatalf("Fatal error: %s", err.Error())
		os.Exit(1)
	}
	childHashes := getChildrenBlocksHashes(block.GetBlockchainHeaders(), block.GetHeight())
	rootHash := getChildrenRootHash(childHashes)
	return olhash.Blake2bl(rootHash.Xor(rootHash, new(big.Int).SetBytes(prevHashBytes)).String())
}

func headerListIsOrderedByHeight(hlist []*p2p_pb.BlockchainHeader) bool {
	n := len(hlist)
	for i := n - 1; i > 0; i-- {
		if hlist[i].GetHeight() < hlist[i-1].GetHeight() {
			return false
		}
	}
	return true
}

func markedTxHash(mtx *p2p_pb.MarkedTransaction) string {
	asJSString := []string{}
	for _, byte := range mtx.GetValue() {
		asJSString = append(asJSString, fmt.Sprintf("%d", byte))
	}
	toHash := fmt.Sprintf(
		"%s%s%s%s%s",
		mtx.GetId(),
		mtx.GetToken(),
		mtx.GetAddrFrom(),
		mtx.GetAddrTo(),
		strings.Join(asJSString, ","),
	)
	//zap.S().Debugf("markedTxHash -> toHash %s", toHash)
	return olhash.Blake2bl(toHash)
}

func blockHeaderHash(header *p2p_pb.BlockchainHeader, height uint64) string {
	const (
		jsHash1                 = "4e0e3c1e3a5fcb4096a53cc0672475740ab4eedb9adb974b84be4c7e9d95dee9"
		nonDeterministicHash1   = "48e97a284e4588e4d08e7a6bfa3c0e311aa929af4b52c533b3ef3eeae2944f57"
		nonDeterministicHeight1 = uint64(1074552)
		jsHash2                 = "83644a084e313a5464cf03f08d4890e723cf2a433aa3b9562c180760b5f6e5ee"
		nonDeterministicHash2   = "ace091c229a84f2db8c62cf1f3920bc3338161143708176d329da5bbe7fcf31d"
		nonDeterministicHeight2 = uint64(2171001)
	)
	toHash := header.GetHash() + header.GetMerkleRoot()
	for _, mtx := range header.GetMarkedTxs() {
		mtxHash := markedTxHash(mtx)
		zap.S().Debugf("hashedMtx -> %v", mtxHash)
		toHash += mtxHash
	}
	//zap.S().Debugf("blockHeaders -> toHash %s", toHash)

	outHash := olhash.Blake2bl(toHash)
	// these serialization differences seem to be non-deterministic in behavior
	// across languages, this will be in different orders for js and golang
	//zap.S().Debugf("blockHeaders -> hashed start %s", outHash)
	if height == nonDeterministicHeight1 && outHash == nonDeterministicHash1 {
		outHash = jsHash1
	}
	if height == nonDeterministicHeight2 && outHash == nonDeterministicHash2 {
		outHash = jsHash2
	}
	//zap.S().Debugf("blockHeaders -> hashed final %s", outHash)

	return outHash
}

func getChildrenBlocksHashes(headers *p2p_pb.BlockchainHeaders, height uint64) []string {
	out := []string{}
	for _, header := range headers.GetBtc() {
		out = append(out, blockHeaderHash(header, height))
	}
	for _, header := range headers.GetEth() {
		out = append(out, blockHeaderHash(header, height))
	}
	for _, header := range headers.GetLsk() {
		out = append(out, blockHeaderHash(header, height))
	}
	for _, header := range headers.GetNeo() {
		out = append(out, blockHeaderHash(header, height))
	}
	for _, header := range headers.GetWav() {
		out = append(out, blockHeaderHash(header, height))
	}
	return out
}

func getChildrenRootHash(hashes []string) *big.Int {
	out := new(big.Int)
	for _, hash := range hashes {
		hashBytes, err := hex.DecodeString(hash)
		if err != nil {
			zap.S().Fatalf("Fatal error: %s", err.Error())
			os.Exit(1)
		}
		out.Xor(out, new(big.Int).SetBytes(hashBytes))
	}
	return out
}

// comments below are copied from:
// https://github.com/trick77/bc-src/blob/10cccce1da73baff056d049f4b92d5ef50f96e6a/lib/bc/validation.js
// they're all a bit wutful
func isSkippableBlock(block *p2p_pb.BcBlock) bool {
	const specialPrevHash = "ed0e95c8175035d15088ae2679f09f82d7b50477ad13d71d417563a5472cd150"
	height := block.GetHeight()
	prevHash := block.GetPreviousHash()

	// block soft opening limit
	if height < 151000 {
		zap.S().Debugf("Block validation skipping block %v < 151000", height)
		return true
	}

	// gpu mining
	if height == 303401 || height == 641452 {
		zap.S().Debugf("Block validation skipping block %v == 303401, 641452", height)
		return true
	}

	if height <= 1074546 && height >= 1074499 {
		zap.S().Debugf("Block validation skipping block 1074499 < %v <= 1074546", height)
		return true
	}

	if height <= 2171032 && height >= 2171009 {
		zap.S().Debugf("Block validation skipping block 2171009 < %v <= 2171032", height)
		return true
	}

	if height == 4207048 {
		zap.S().Debugf("Block validation skipping block %v == 4207048", height)
		return true
	}

	// overline weighted txs
	if height == 1771221 && prevHash == specialPrevHash {
		zap.S().Debugf("Block validation skipping block %v == 1771221, %v == %v", height, prevHash, specialPrevHash)
		return true
	}
	return false
}
