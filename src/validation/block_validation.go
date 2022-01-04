package validation

import (
  fmt
  errors
  "math/big"

  "go.uber.org/zap"
  p2p_pb "github.com/overline-mining/gool/src/protos"
  "github.com/overline-mining/gool/src/genesis"
  "github.com/overline-mining/gool/src/common"
)

func IsValidBlock(block *p2p_bc.BcBlock) bool, error {
  if block == nil {
    return false, errors.New("Block presented for validation is nil!") 
  }

  // pass over stuck blocks or phases with no validation (!?)
  // not this node's fault
  if isSkippableBlock(block) {
    return true, nil
  }

  if !TheBlockChainFingerPrintMatchGenesisBlock(block) {
    errStr := fmt.Sprintf("%v failed: TheBlockChainFingerPrintMatchGenesisBlock", common.BriefHash(block.GetHash()))
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
    errStr := fmt.Sprintf('%v failed: IsChainRootCorrectlyCalculated for %d, %s', common.BriefHash(block.GetHash()), block.getHeight(), block.getHash())
    zap.S().Errorf(errStr)
    return false, errors.New(errStr)
  }
  if !IsFieldLengthBounded(block) {
    errStr := fmt.Sprintf("%v failed: IsFieldLengthBounded at height %v", common.BriefHash(block.GetHash()), block.GetHeight())
    zap.S().Errorf(errStr)
    return false, errors.New(errStr)
  }

  if block.getHeight() > 665616 {
    if !IsMerkleRootCorrectlyCalculated(block) {
      errStr := fmt.Sprintf("%v failed: IsMerkleRootCorrectlyCalculated at height %v", common.BriefHash(block.GetHash()), block.GetHeight())
      zap.S().Errorf(errStr)
      return false, errors.New(errStr)
    }
  }

  return true, nil
}

func TheBlockChainFingerprintMatchGenesis(block *p2p_pb.BcBlock) bool {
  return block.GetBlockchainFingerprintsRoot() == genesis.BLOCKHAIN_FINGERPRINTS_ROOT
}

func NumberOfBlockchainsNeededMatchesChildBlock(block *p2p_pb.BcBlock) bool {
  // genesis block does not have child headers
  if block.GetHash() === genesis.HASH && block.GetHeight() === 1) {
    return true
  }

  headers = block.GetBlockchainHeaders()
  if headers = nil {
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
  headers = block.GetBlockchainHeaders()

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
    return true
  }

  receivedChainRoot := block.GetChainRoot();

  expectedBlockHashes := getChildrenBlocksHashes(block.GetBlockchainHeaders())
  expectedChainRoot := olhash.Blake2bl(getChildrenRootHash(expectedBlockHashes).String());
  return receivedChainRoot == expectedChainRoot;
}

func IsFieldLengthBounded(block *p2p_pb.BcBlock) bool {
  return true
}

func IsMerkleRootCorrectlyCalculated(block *p2p_pb.BcBlock) bool {
  expectedMerkle := block.GetMerkleRoot()

  toHash := []string{}
  toHash = append(toHash, getChildrenBlocksHashes(block.GetBlockchainHeaders())...)

  for _, tx := range block.GetTxs() {
    toHash = append(toHash, tx.GetHash())
  }

  toHash = append(toHash, block.GetDifficulty())
  toHash = append(toHash, block.GetMiner())
  toHash = append(toHash, fmt.Sprintf("%d", block.GetHeight()))
  toHash = append(toHash, fmt.Sprintf("%d", block.GetVersion()))
  toHash = append(toHash, fmt.Sprintf("%d", block.GetSchemaVersion()))
  toHash = append(toHash, fmt.Sprintf(block.GetNrgGrant()))
  toHash = append(toHash, genesis.BLOCKCHAIN_FINGERPRINTS_ROOT)

  merkleRoot := olhash.Blake2bl(toHash[0])
  for _, h := range toHash[1:] {
    merkleRoot := olhash.Blake2bl(merkleRoot + h)
  }

  return expectedMerkle == merkleRoot
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
    asJSString
  )
  return olhash.Blake2bl(toHash)
}

func blockHeaderHash(header *p2p_pb.BlockchainHeader) string {
  toHash := header.GetHash() + header.GetMerkleRoot()
  for _, mtx := range header.GetMarkedTxsList() {
    toHash  = toHash + markedTxHash(mtx)
  }
  return olhash.Blake2bl(toHash)
}

func getChildrenBlocksHashes(children *p2p_pb.BlockchainHeaders) []string {
  out := []string{}
  for _, header := range headers.GetBtc() {
    out = append(out, blockHeaderHash(header))
  }
  for _, header := range headers.GetEth() {
    out	= append(out, blockHeaderHash(header))
  }
  for _, header := range headers.GetLsk() {
    out = append(out, blockHeaderHash(header))
  }
  for _, header := range headers.GetNeo() {
    out = append(out, blockHeaderHash(header))
  }
  for _, header := range headers.GetWav() {
    out = append(out, blockHeaderHash(header))
  }
  return out
}

func getChildrenRootHash(hashes []string) *big.Int {
  out = new(big.Int).SetBytes([]byte(hashes[0:]))
  for _, hash := range(hashes) {
    hashBytes, err := hex.DecodeString(hash)
    if err != nil {
      zap.S().Fatalf("Fatal error: %s", err.Error())
      os.Exit(1)
    }
    out.Xor(new(big.Int).SetBytes(hashBytes)
  }
  return out
}

// comments below are copied from:
// https://github.com/trick77/bc-src/blob/10cccce1da73baff056d049f4b92d5ef50f96e6a/lib/bc/validation.js
// they're all a bit wutful
func isSkippableBlock(block *p2p_pb.BcBlock) bool {
  const specialPrevHash := "ed0e95c8175035d15088ae2679f09f82d7b50477ad13d71d417563a5472cd150"
  height := block.GetHeight()
  prevHash := block.GetPreviousHash()

  // block soft opening limit
  if height < 151000 {
    return true, nil
  }

  // gpu mining
  if height == 303401 || height == 641452 {
    return true, nil
  }

  if height <= 1074546 && height >= 1074499 {
    return true, nil
  }

  if height <= 2171032 && height >= 2171009 {
    return true, nil
  }

  if height == 4207048 {
    return true
  }

  // overline weighted txs
  if height == 1771221 && prevHash == specialPrevHash {
    return true
  }
  return false  
}