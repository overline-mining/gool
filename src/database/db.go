package database

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	"github.com/overline-mining/gool/src/common"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"github.com/overline-mining/gool/src/validation"
	lz4 "github.com/pierrec/lz4/v4"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"math/big"
	"sort"
	"sync"
	"time"

	probar "github.com/schollz/progressbar/v3"
)

var ChainstateBlocks = []byte("CHAINSTATE-BLOCKS")
var ChainstateTxs = []byte("CHAINSTATE-TXS")
var ChainstateMTxs = []byte("CHAINSTATE-MTXS")
var OverlineBlockChunks = []byte("OVERLINE-BLOCK-CHUNKS")
var OverlineBlockChunkMap = []byte("OVERLINE-BLOCK-CHUNK-MAP")
var OverlineHeightToHashMap = []byte("OVERLINE-BLOCK-HEIGHT-TO-HASH")
var OverlineTxToHashMap = []byte("OVERLINE-TX-TO-BLOCK")
var SyncInfo = []byte("SYNC-INFO")

type OverlineDBConfig struct {
	Maturity           int `json:"maturity"`           // how many blocks until maturity is reached
	IncomingBlocksSize int `json:"incomingBlocksSize"` // how big is the map of incoming blocks
	ActiveSet          int `json:"activeSet"`          // how many blocks past maturity to keep in memory
	AncientChunkSize   int `json:"ancientChunkSize"`   // how many blocks to serialize to disk together
}

// LRU(Maturity + ActiveSet) is our in-memory block-cache

func DefaultOverlineDBConfig() OverlineDBConfig {
	return OverlineDBConfig{
		Maturity:           100,
		IncomingBlocksSize: 1000,
		ActiveSet:          100,
		AncientChunkSize:   1000,
	}
}

func orderedPairPrint(first, second *p2p_pb.BcBlock) {
	zap.S().Debugf(
		"Ordered block pair check: %v (%v, %v) -> %v (%v, %v)",
		first.GetHeight(),
		common.BriefHash(first.GetHash()),
		common.BriefHash(first.GetPreviousHash()),
		second.GetHeight(),
		common.BriefHash(second.GetHash()),
		common.BriefHash(second.GetPreviousHash()),
	)
}

func orderedPairCheck(first, second *p2p_pb.BcBlock) {
	if !validation.OrderedBlockPairIsValid(first, second) {
		common.CheckError(
			errors.New(
				fmt.Sprintf(
					"%v (%v, %v) -> %v (%v, %v) Invalid pair!",
					first.GetHeight(),
					common.BriefHash(first.GetHash()),
					common.BriefHash(first.GetPreviousHash()),
					second.GetHeight(),
					common.BriefHash(second.GetHash()),
					common.BriefHash(second.GetPreviousHash()),
				)))
	}
}

type OverlineDB struct {
	Config                 OverlineDBConfig
	db                     *bolt.DB
	mu                     sync.Mutex
	txMu                   sync.Mutex
	ibdMu                  sync.Mutex
	ibdMode                bool              // Initial Block Download mode
	multiplexPeers         bool              // when in IBD how to we query blocks from peers (stripe over peers (true) or request the same blocks from each (false))
	syncStartingBlock      *p2p_pb.BcBlock   // the block we started syncing from in this run of gool
	tipOfSerializedChain   *p2p_pb.BcBlock   // the highest, main-chain serialized block
	highestBlock           *p2p_pb.BcBlock   // the highest block awaiting serialization
	toSerialize            []*p2p_pb.BcBlock // sorted ascending in block height
	incomingBlocks         map[string]*p2p_pb.BcBlock
	incomingBlocksByHeight map[uint64]string // map of height -> hash
	incomingBlocksByTx     map[string]string // map of tx hash -> hash
	txMemPool              map[string]*p2p_pb.Transaction
	mtxMemPool             map[string]*p2p_pb.MarkedTransaction
	lookupCache            *lru.ARCCache
	blockStripeCache       *lru.ARCCache
}

func (odb *OverlineDB) Open(filepath string, dropChainstate bool, pruneDatabaseTo uint64) error {
	var err error
	odb.db, err = bolt.Open(filepath, 0600, nil)
	odb.lookupCache, err = lru.NewARC(5 * odb.Config.ActiveSet)
	odb.blockStripeCache, err = lru.NewARC(odb.Config.ActiveSet)
	odb.mu.Lock()
	odb.toSerialize = make([]*p2p_pb.BcBlock, 0, 10*odb.Config.AncientChunkSize)
	odb.incomingBlocks = make(map[string]*p2p_pb.BcBlock)
	odb.incomingBlocksByHeight = make(map[uint64]string)
	odb.incomingBlocksByTx = make(map[string]string)
	odb.txMu.Lock()
	odb.txMemPool = make(map[string]*p2p_pb.Transaction)
	odb.mtxMemPool = make(map[string]*p2p_pb.MarkedTransaction)
	odb.txMu.Unlock()

	if dropChainstate {
		err = odb.db.Update(func(tx *bolt.Tx) error {
			err := tx.DeleteBucket(ChainstateBlocks)
			if err != nil {
				zap.S().Error(err)
			}
			err = tx.DeleteBucket(ChainstateTxs)
			if err != nil {
				zap.S().Error(err)
			}
			err = tx.DeleteBucket(ChainstateMTxs)
			if err != nil {
				zap.S().Error(err)
			}

			_, err = tx.CreateBucket(ChainstateBlocks)
			if err != nil {
				zap.S().Error(err)
			}
			_, err = tx.CreateBucket(ChainstateTxs)
			if err != nil {
				zap.S().Error(err)
			}
			_, err = tx.CreateBucket(ChainstateMTxs)
			if err != nil {
				zap.S().Error(err)
			}
			return err
		})
	}

	if pruneDatabaseTo > 0 {
		odb.deleteBlocksAfter(pruneDatabaseTo)
	}

	err = odb.db.View(func(tx *bolt.Tx) error {
		syncInfo := tx.Bucket(SyncInfo)
		if syncInfo != nil {
			lastWrittenHash := syncInfo.Get([]byte("LastWrittenBlockHash"))
			odb.tipOfSerializedChain, err = odb.getSerializedBlock(lastWrittenHash)
			if err != nil {
				return err
			}
			odb.highestBlock = odb.tipOfSerializedChain
		} else {
			return errors.New("Blockchain database malformed SYNC-INFO bucket not available!")
		}
		return nil
	})

	err = odb.db.View(func(tx *bolt.Tx) error {
		blocks := tx.Bucket(ChainstateBlocks)
		txs := tx.Bucket(ChainstateTxs)
		mtxs := tx.Bucket(ChainstateMTxs)
		if blocks == nil {
			return errors.New("Uninitialized blockchain file!")
		}
		odb.ibdMu.Lock()
		odb.ibdMode = true
		odb.ibdMu.Unlock()
		cblocks := blocks.Cursor()
		goodBlocks := new(p2p_pb.BcBlocks)
		for hash, serblock := cblocks.First(); hash != nil; hash, serblock = cblocks.Next() {
			block := new(p2p_pb.BcBlock)
			err := proto.Unmarshal(serblock, block)
			isValid, _ := validation.IsValidBlock(block)
			if err == nil && isValid {
				goodBlocks.Blocks = append(goodBlocks.Blocks, block)
			}
		}
		sort.SliceStable(goodBlocks.Blocks, func(i, j int) bool {
			if goodBlocks.Blocks[i].GetHeight() == goodBlocks.Blocks[j].GetHeight() {
				iDist, _ := new(big.Int).SetString(goodBlocks.Blocks[i].GetTotalDistance(), 10)
				jDist, _ := new(big.Int).SetString(goodBlocks.Blocks[j].GetTotalDistance(), 10)
				compare := iDist.Cmp(jDist)
				if compare == 0 {
					return goodBlocks.Blocks[i].GetTimestamp() < goodBlocks.Blocks[j].GetTimestamp()
				}
				return compare < 0
			}
			return goodBlocks.Blocks[i].GetHeight() < goodBlocks.Blocks[j].GetHeight()
		})
		for _, b := range goodBlocks.Blocks {
			odb.addBlockUnsafe(b)
		}
		odb.txMu.Lock()
		curtx := txs.Cursor()
		for hash, sertx := curtx.First(); hash != nil; hash, sertx = curtx.Next() {
			strhash := hex.EncodeToString(hash)
			tx := new(p2p_pb.Transaction)
			err := proto.Unmarshal(sertx, tx)
			if err == nil {
				odb.txMemPool[strhash] = tx
			}
		}
		curmtx := mtxs.Cursor()
		for hash, sermtx := curmtx.First(); hash != nil; hash, sermtx = curmtx.Next() {
			strhash := hex.EncodeToString(hash)
			mtx := new(p2p_pb.MarkedTransaction)
			err := proto.Unmarshal(sermtx, mtx)
			if err == nil {
				odb.mtxMemPool[strhash] = mtx
			}
		}
		odb.txMu.Unlock()

		return nil
	})

	if err != nil {
		odb.tipOfSerializedChain = nil
		odb.highestBlock = nil
	} else {
		zap.S().Infof("Recovered last serialized block    : %v -> %v", common.BriefHash(odb.tipOfSerializedChain.GetHash()), odb.tipOfSerializedChain.GetHeight())
		zap.S().Infof("Recovered highest contiguous block : %v -> %v", common.BriefHash(odb.highestBlock.GetHash()), odb.highestBlock.GetHeight())
		odb.syncStartingBlock = odb.highestBlock
	}

	if odb.tipOfSerializedChain != nil {
		hashes_to_kill := make([]string, 0)
		heights_to_kill := make([]uint64, 0)
		txs_to_kill := make([]string, 0)
		for _, block := range odb.incomingBlocks {
			if block.GetHeight() <= odb.tipOfSerializedChain.GetHeight() {
				hashes_to_kill = append(hashes_to_kill, block.GetHash())
				heights_to_kill = append(heights_to_kill, block.GetHeight())
				for _, tx := range block.GetTxs() {
					txs_to_kill = append(txs_to_kill, tx.GetHash())
				}
			}
		}
		for _, hash := range hashes_to_kill {
			delete(odb.incomingBlocks, hash)
			zap.S().Debugf("Killing: %v", common.BriefHash(hash))
		}
		for _, height := range heights_to_kill {
			delete(odb.incomingBlocksByHeight, height)
			zap.S().Debugf("Killing: %v", height)
		}
		for _, tx := range txs_to_kill {
			delete(odb.incomingBlocksByTx, tx)
			zap.S().Debugf("Killing: %v", tx)
		}
	}

	odb.SetInitialBlockDownload()
	odb.SetMultiplexPeers()
	odb.mu.Unlock()
	return err
}

func (odb *OverlineDB) Close() {
	odb.FlushToDisk() // write whatever is in memory to disk
	odb.mu.Lock()
	odb.txMu.Lock()
	// zero out all buffers and maps
	odb.toSerialize = make([]*p2p_pb.BcBlock, 0, 0)
	odb.incomingBlocks = make(map[string]*p2p_pb.BcBlock)
	odb.txMemPool = make(map[string]*p2p_pb.Transaction)
	odb.mtxMemPool = make(map[string]*p2p_pb.MarkedTransaction)
	odb.txMu.Unlock()
	odb.mu.Unlock()
	odb.db.Close()
}

func (odb *OverlineDB) addBlockUnsafe(block *p2p_pb.BcBlock) bool {
	if _, ok := odb.incomingBlocks[block.GetHash()]; !ok {
		odb.incomingBlocks[block.GetHash()] = block
		odb.incomingBlocksByHeight[block.GetHeight()] = block.GetHash()
		for _, tx := range block.GetTxs() {
			odb.incomingBlocksByTx[tx.GetHash()] = block.GetHash()
		}
		if odb.highestBlock == nil && block.GetHeight() == 1 {
			odb.highestBlock = block
		} else {
			blockDist, _ := new(big.Int).SetString(block.GetTotalDistance(), 10)
			highestDist, _ := new(big.Int).SetString(odb.highestBlock.GetTotalDistance(), 10)
			if block.GetPreviousHash() == odb.highestBlock.GetHash() &&
				block.GetHeight() == odb.highestBlock.GetHeight()+1 {
				odb.highestBlock = block
			} else if block.GetPreviousHash() == odb.highestBlock.GetPreviousHash() &&
				blockDist.Cmp(highestDist) > 0 {
				odb.highestBlock = block
			}
		}
	} else {
		zap.S().Debugf("Block %v:%v already seen.", block.GetHeight(), common.BriefHash(block.GetHash()))
		return false // not necessary to add block
	}
	return true // block was added
}

func (odb *OverlineDB) SetMultiplexPeers() {
	odb.ibdMu.Lock()
	odb.multiplexPeers = true
	odb.ibdMu.Unlock()
}

func (odb *OverlineDB) UnSetMultiplexPeers() {
	odb.ibdMu.Lock()
	odb.multiplexPeers = false
	odb.ibdMu.Unlock()
}

func (odb *OverlineDB) IsMultiplexPeers() bool {
	odb.ibdMu.Lock()
	out := odb.multiplexPeers
	odb.ibdMu.Unlock()
	return out
}

func (odb *OverlineDB) SetInitialBlockDownload() {
	odb.ibdMu.Lock()
	odb.ibdMode = true
	odb.ibdMu.Unlock()
}

func (odb *OverlineDB) UnSetInitialBlockDownload() {
	odb.ibdMu.Lock()
	odb.ibdMode = false
	odb.ibdMu.Unlock()
}

func (odb *OverlineDB) IsInitialBlockDownload() bool {
	odb.ibdMu.Lock()
	out := odb.ibdMode
	odb.ibdMu.Unlock()
	return out
}

func (odb *OverlineDB) SerializedHeight() uint64 {
	odb.mu.Lock()
	defer odb.mu.Unlock()
	if odb.tipOfSerializedChain != nil {
		return odb.tipOfSerializedChain.GetHeight()
	}
	return 0
}

func (odb *OverlineDB) HighestSerializedBlock() p2p_pb.BcBlock {
	odb.mu.Lock()
	defer odb.mu.Unlock()
	block := *odb.tipOfSerializedChain
	return block
}

func (odb *OverlineDB) HighestBlockHeight() uint64 {
	odb.mu.Lock()
	defer odb.mu.Unlock()
	if odb.highestBlock != nil {
		return odb.highestBlock.GetHeight()
	}
	return 0
}

func (odb *OverlineDB) HighestBlock() *p2p_pb.BcBlock {
	odb.mu.Lock()
	defer odb.mu.Unlock()
	return odb.highestBlock
}

func (odb *OverlineDB) AddBlock(block *p2p_pb.BcBlock) (bool, error) {
	isValid, err := validation.IsValidBlock(block)
	if !isValid {
		return false, err
	}
	// at this point the block is known to be valid
	testHashBytes, _ := hex.DecodeString(block.GetHash())
	testDbBlock, _ := odb.GetBlockByHash(testHashBytes)

	if testDbBlock != nil && testDbBlock.GetHash() == block.GetHash() {
		return false, errors.New(fmt.Sprintf("Block %v already in database", common.BriefHash(block.GetHash())))
	}

	odb.mu.Lock()
	added := odb.addBlockUnsafe(block)
	odb.mu.Unlock()
	return added, nil
}

func (odb *OverlineDB) AddBlockRange(brange *p2p_pb.BcBlocks) (int, error) {
	added := 0
	for _, block := range brange.Blocks {
		isValid, err := validation.IsValidBlock(block)
		if !isValid {
			return added, err
		}
		testHashBytes, _ := hex.DecodeString(block.GetHash())
		testDbBlock, _ := odb.GetBlockByHash(testHashBytes)
		if testDbBlock != nil && testDbBlock.GetHash() == block.GetHash() {
			return added, errors.New(fmt.Sprintf("Block %v already in database", common.BriefHash(block.GetHash())))
		}
		odb.mu.Lock()
		if odb.addBlockUnsafe(block) {
			added++
		}
		odb.mu.Unlock()
	}
	return added, nil
}

func (odb *OverlineDB) AddTransaction(tx *p2p_pb.Transaction) {
	odb.txMu.Lock()
	odb.txMemPool[tx.GetHash()] = tx
	odb.txMu.Unlock()
}

func (odb *OverlineDB) AddMarkedTransaction(mtx *p2p_pb.MarkedTransaction) {
	odb.txMu.Lock()
	odb.mtxMemPool[mtx.GetHash()] = mtx
	odb.txMu.Unlock()
}

func (odb *OverlineDB) GetBlockByHeight(blockHeight uint64) (*p2p_pb.BcBlock, error) {
	tryblock, ok1 := odb.lookupCache.Get(blockHeight)
	block, ok2 := tryblock.(*p2p_pb.BcBlock)
	var err error = nil
	if !(ok1 && ok2) {
		// three places to look for a block:
		// incomingBlocks (needs lock)
		odb.mu.Lock()
		blockHash, ok1 := odb.incomingBlocksByHeight[blockHeight]
		if ok1 {
			block, ok1 = odb.incomingBlocks[blockHash]
		}
		// toSerialize (also needs lock)
		if !ok1 {
			idx := sort.Search(len(odb.toSerialize), func(i int) bool { return odb.toSerialize[i].GetHeight() >= blockHeight })
			if idx < len(odb.toSerialize) && odb.toSerialize[idx].GetHeight() == blockHeight {
				ok1 = true
				block = odb.toSerialize[idx]
			}
		}
		odb.mu.Unlock()
		if !ok1 {
			seekBytes := make([]byte, 8)
			var hashBytes []byte
			binary.BigEndian.PutUint64(seekBytes, blockHeight)
			err := odb.db.View(func(tx *bolt.Tx) error {
				height2hash := tx.Bucket(OverlineHeightToHashMap)
				hashBytes = height2hash.Get(seekBytes)
				if len(hashBytes) == 0 {
					return errors.New(fmt.Sprintf("Could not find height %v!", blockHeight))
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			block, err = odb.GetBlockByHash(hashBytes)
		}
		if err == nil {
			odb.lookupCache.Add(blockHeight, block)
			odb.lookupCache.Add(block.GetHash(), block)
			for _, tx := range block.GetTxs() {
				odb.lookupCache.Add(tx.GetHash(), block)
			}
		}
	}
	return block, err
}

func (odb *OverlineDB) GetBlockByTx(txHash []byte) (*p2p_pb.BcBlock, error) {
	txString := hex.EncodeToString(txHash)
	tryblock, ok1 := odb.lookupCache.Get(txString)
	block, ok2 := tryblock.(*p2p_pb.BcBlock)
	var err error = nil
	if !(ok1 && ok2) {
		// three places to look for a block:
		// incomingBlocks (needs lock)
		odb.mu.Lock()
		blockHash, ok1 := odb.incomingBlocksByTx[txString]
		if ok1 {
			block, ok1 = odb.incomingBlocks[blockHash]
		}
		// toSerialize (also needs lock)
		// FIXME - this needs to be better than a linear search
		if !ok1 {
			for _, blk := range odb.toSerialize {
				for _, tx := range blk.GetTxs() {
					if tx.GetHash() == txString {
						ok1 = true
						block = blk
						break
					}
				}
				if ok1 {
					break
				}
			}
		}
		odb.mu.Unlock()
		if !ok1 {
			hashBytes := make([]byte, 0)
			err := odb.db.View(func(tx *bolt.Tx) error {
				tx2hash := tx.Bucket(OverlineTxToHashMap)
				hashBytes = tx2hash.Get(txHash)
				if len(hashBytes) == 0 {
					return errors.New(fmt.Sprintf("Could not find tx %v!", txHash))
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			block, err = odb.GetBlockByHash(hashBytes)
		}
		if err == nil {
			odb.lookupCache.Add(txHash, block)
			odb.lookupCache.Add(block.GetHeight(), block)
			odb.lookupCache.Add(block.GetHash(), block)
		}
	}
	return block, err
}

func (odb *OverlineDB) GetBlockByHash(blockHash []byte) (*p2p_pb.BcBlock, error) {
	blockHashString := hex.EncodeToString(blockHash)
	tryblock, ok1 := odb.lookupCache.Get(blockHashString)
	block, ok2 := tryblock.(*p2p_pb.BcBlock)
	var err error = nil
	if !(ok1 && ok2) {
		// three places to look for a block:
		// incomingBlocks (needs lock)
		odb.mu.Lock()
		block, ok1 = odb.incomingBlocks[blockHashString]
		// toSerialize (also needs lock)
		// FIXME - this needs to be better than a linear search
		if !ok1 {
			for _, b := range odb.toSerialize {
				if b.GetHash() == blockHashString {
					block = b
					ok1 = true
					break
				}
			}
		}
		odb.mu.Unlock()
		if !ok1 {
			// serialized in database (doesn't need lock)
			block, err = odb.getSerializedBlock(blockHash)
			if err != nil {
				return nil, err
			}
		}
		odb.lookupCache.Add(blockHashString, block)
		odb.lookupCache.Add(block.GetHeight(), block)
		for _, tx := range block.GetTxs() {
			odb.lookupCache.Add(tx.GetHash(), block)
		}
	}
	return block, nil
}

func (odb *OverlineDB) getSerializedBlock(blockHash []byte) (*p2p_pb.BcBlock, error) {
	blockHashString := hex.EncodeToString(blockHash)
	block := new(p2p_pb.BcBlock)
	ok := false

	err := odb.db.View(func(tx *bolt.Tx) error {
		chunks := tx.Bucket([]byte("OVERLINE-BLOCK-CHUNKS"))
		block2chunk := tx.Bucket([]byte("OVERLINE-BLOCK-CHUNK-MAP"))

		chunkHash := block2chunk.Get(blockHash)
		chunkHashStr := hex.EncodeToString(chunkHash)
		decompedIface, chunkInCache := odb.lookupCache.Get(chunkHashStr)
		decomped, castOk := decompedIface.([]byte)
		if !(chunkInCache && castOk) {
			chunk := chunks.Get(chunkHash)
			decompressionBuf := make([]byte, 10*len(chunk))
			nDecompressed, err := lz4.UncompressBlock(chunk, decompressionBuf)
			if err != nil {
				return err
			}
			decomped = make([]byte, nDecompressed, nDecompressed)
			copy(decomped, decompressionBuf[:nDecompressed])
			odb.lookupCache.Add(chunkHashStr, decomped)
		}
		blockList := p2p_pb.BcBlocks{}
		err := proto.Unmarshal(decomped, &blockList)
		if err != nil {
			return err
		}
		for _, chunkBlock := range blockList.Blocks {
			if chunkBlock.GetHash() == blockHashString {
				block = chunkBlock
				ok = true
				break
			}
		}

		if !ok {
			return errors.New(fmt.Sprintf("Block %v not found in serialized block database!", blockHashString))
		}

		return nil
	})
	return block, err
}

func (odb *OverlineDB) FlushToDisk() {
	odb.mu.Lock()
	odb.txMu.Lock()

	if len(odb.incomingBlocks) > odb.Config.AncientChunkSize {
		odb.runSerialization()
	}

	err := odb.db.Update(func(tx *bolt.Tx) error {
		chainstate_blocks := tx.Bucket(ChainstateBlocks)
		chainstate_txs := tx.Bucket(ChainstateTxs)
		chainstate_mtxs := tx.Bucket(ChainstateMTxs)

		// reset on-disk chainstates
		if chainstate_blocks != nil {
			tx.DeleteBucket(ChainstateBlocks)
		}
		chainstate_blocks, _ = tx.CreateBucket(ChainstateBlocks)
		if chainstate_txs != nil {
			tx.DeleteBucket(ChainstateTxs)
		}
		chainstate_txs, _ = tx.CreateBucket(ChainstateTxs)
		if chainstate_mtxs != nil {
			tx.DeleteBucket(ChainstateMTxs)
		}
		chainstate_mtxs, _ = tx.CreateBucket(ChainstateMTxs)
		// fill the chainstate
		for hash, block := range odb.incomingBlocks {
			key, _ := hex.DecodeString(hash)
			blockBytes, _ := proto.Marshal(block)
			err := chainstate_blocks.Put(key, blockBytes)
			if err != nil {
				return err
			}
		}
		for hash, tx := range odb.txMemPool {
			key, _ := hex.DecodeString(hash)
			txBytes, _ := proto.Marshal(tx)
			err := chainstate_txs.Put(key, txBytes)
			if err != nil {
				return err
			}
		}
		for hash, mtx := range odb.mtxMemPool {
			key, _ := hex.DecodeString(hash)
			mtxBytes, _ := proto.Marshal(mtx)
			err := chainstate_txs.Put(key, mtxBytes)
			if err != nil {
				return err
			}
		}
		return nil
	})
	common.CheckError(err)
	odb.txMu.Unlock()
	odb.mu.Unlock()
}

func (odb *OverlineDB) Run() {
	// start a thread that moves incoming blocks
	// into the serialization buffer
	go func() {
		for {
			odb.mu.Lock()
			if len(odb.incomingBlocks) > odb.Config.IncomingBlocksSize {
				odb.runSerialization()
				odb.mu.Unlock()
			} else {
				odb.mu.Unlock()
				time.Sleep(time.Second * 2)
			}
		}
	}()
	// Run itself handles serialization,
	// we wait for the block height to progress

}

func (odb *OverlineDB) FullLocalValidation() {
	odb.mu.Lock()
	decompressionBuf := make([]byte, 0x3000000) // 50MB should be enough to cover
	err := odb.db.View(func(tx *bolt.Tx) error {
		heights := tx.Bucket(OverlineHeightToHashMap)
		chunks := tx.Bucket(OverlineBlockChunks)
		block2chunk := tx.Bucket(OverlineBlockChunkMap)

		c := heights.Cursor()

		lastHeight := odb.tipOfSerializedChain.GetHeight()
		bar := probar.Default(int64(lastHeight), "full validation ->")

		seekBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(seekBytes, uint64(1))

		currentChunk := make([]byte, 32)
		blockMap := make(map[string]*p2p_pb.BcBlock)
		for k, v := c.Seek(seekBytes); k != nil; k, v = c.Next() {
			height := binary.BigEndian.Uint64(k)
			hash := hex.EncodeToString(v)
			chunkHash := block2chunk.Get(v)
			chunk := chunks.Get(chunkHash)

			if bytes.Compare(currentChunk, chunkHash) != 0 {
				blockMap = make(map[string]*p2p_pb.BcBlock) // reset the blockmap
				nDecompressed, err := lz4.UncompressBlock(chunk, decompressionBuf[0:])
				if err != nil {
					return err
				}
				blockList := p2p_pb.BcBlocks{}
				err = proto.Unmarshal(decompressionBuf[:nDecompressed], &blockList)
				if err != nil {
					return err
				}
				for _, block := range blockList.Blocks {
					blockMap[block.GetHash()] = block
				}
				strCurrentChunk := hex.EncodeToString(currentChunk)
				strChunkHash := hex.EncodeToString(chunkHash)
				zap.S().Debugf("Updating chunk hash from %s to %s", common.BriefHash(strCurrentChunk), common.BriefHash(strChunkHash))
				copy(currentChunk, chunkHash)
			}
			block := blockMap[hash]
			isValid, err := validation.IsValidBlock(block)
			if err != nil {
				zap.S().Errorf("%v: %v", common.BriefHash(hash), err)
			}
			if isValid {
				var prevBlock *p2p_pb.BcBlock
				if _, ok := blockMap[block.GetPreviousHash()]; !ok {
					zap.S().Debugf("Block's previous hash %v was not in chunk!", block.GetPreviousHash())
					temp := p2p_pb.BcBlocks{}
					prevKey, err := hex.DecodeString(block.GetPreviousHash())
					prevChunkHash := block2chunk.Get(prevKey)
					prevChunk := chunks.Get(prevChunkHash)
					nDecompressed, err := lz4.UncompressBlock(prevChunk, decompressionBuf[0:])
					if err != nil {
						return err
					}
					err = proto.Unmarshal(decompressionBuf[:nDecompressed], &temp)
					if err != nil {
						return err
					}
					for _, blk := range temp.Blocks {
						if blk.GetHash() == block.GetPreviousHash() {
							zap.S().Debugf("Found previous hash -> %v", blk.GetHash())
							prevBlock = blk
							break
						}
					}
				} else {
					prevBlock = blockMap[block.GetPreviousHash()]
				}
				if prevBlock == nil {
					if block.GetHeight() == 1 {
						zap.S().Debugf("Genesis block does not have a previous block to find.")
					} else {
						zap.S().Warnf("Could not find previous hash for block %v", block.GetHash())
					}
					continue
				}
				if !validation.OrderedBlockPairIsValid(prevBlock, block) {
					errstr := fmt.Sprintf("%v -> %v does not form a valid chain", prevBlock.GetHash(), block.GetHash())
					zap.S().Debug(errstr)
					return errors.New(errstr)
				}
			} else {
				zap.S().Debugf("Invalid block %v has height %v, expecting %v", common.BriefHash(block.GetHash()), block.GetHeight(), height)
				return err
			}
			bar.Add(1)
		}
		return nil
	})
	common.CheckError(err)
	odb.mu.Unlock()
}

func (odb *OverlineDB) runSerialization() {
	for hash, newBlock := range odb.incomingBlocks {
		odb.toSerialize = append(odb.toSerialize, newBlock)
		delete(odb.incomingBlocks, hash)
		delete(odb.incomingBlocksByHeight, newBlock.GetHeight())
		for _, tx := range newBlock.GetTxs() {
			delete(odb.incomingBlocksByTx, tx.GetHash())
		}
	}
	sort.SliceStable(odb.toSerialize, func(i, j int) bool {
		return common.BlockOrderingRule(odb.toSerialize[i], odb.toSerialize[j])
	})

	if len(odb.toSerialize) > odb.Config.AncientChunkSize {
		toSerialize := odb.toSerialize[:odb.Config.AncientChunkSize]
		if odb.tipOfSerializedChain != nil {
			orderedPairCheck(odb.tipOfSerializedChain, toSerialize[0])
		}
		for iblk := 0; iblk < len(toSerialize)-1; iblk++ {
			orderedPairPrint(toSerialize[iblk], toSerialize[iblk+1])
			orderedPairCheck(toSerialize[iblk], toSerialize[iblk+1])
		}
		odb.serializeBlocks(toSerialize)
		odb.tipOfSerializedChain = toSerialize[odb.Config.AncientChunkSize-1]
		zap.S().Debugf("Set tipOfSerializedChain to: %v %v", odb.tipOfSerializedChain.GetHeight(), common.BriefHash(odb.tipOfSerializedChain.GetHash()))
		// add unserialized blocks back to incomingBlocks
		for _, block := range odb.toSerialize[odb.Config.AncientChunkSize:] {
			odb.incomingBlocks[block.GetHash()] = block
			odb.incomingBlocksByHeight[block.GetHeight()] = block.GetHash()
			for _, tx := range block.GetTxs() {
				odb.incomingBlocksByTx[tx.GetHash()] = block.GetHash()
			}
		}
		odb.toSerialize = make([]*p2p_pb.BcBlock, 0, 10*odb.Config.AncientChunkSize)
	}
}

func (odb *OverlineDB) serializeBlocks(inblocks []*p2p_pb.BcBlock) error {
	c := &lz4.CompressorHC{}
	blocks := p2p_pb.BcBlocks{Blocks: inblocks}
	blocksBytes, err := proto.Marshal(&blocks)
	compressionBuf := make([]byte, len(blocksBytes))
	common.CheckError(err)
	zap.S().Debugf("Blocklist: is %v bytes long, consisting of %v blocks", len(blocksBytes), len(blocks.Blocks))
	nCompressed, err := c.CompressBlock(blocksBytes, compressionBuf)
	common.CheckError(err)
	zap.S().Debugf("Blocklist: compressed to %v bytes!", nCompressed)

	return odb.db.Batch(func(tx *bolt.Tx) error {
		// block chunks keyed by the first hash in the chunk
		chunks, err := tx.CreateBucketIfNotExists([]byte("OVERLINE-BLOCK-CHUNKS"))
		if err != nil {
			return err
		}
		// block hashes to chunks that store that block
		block2chunk, err := tx.CreateBucketIfNotExists([]byte("OVERLINE-BLOCK-CHUNK-MAP"))
		if err != nil {
			return err
		}
		// block height to block hash
		height2hash, err := tx.CreateBucketIfNotExists([]byte("OVERLINE-BLOCK-HEIGHT-TO-HASH"))
		if err != nil {
			return err
		}
		tx2hash, err := tx.CreateBucketIfNotExists([]byte("OVERLINE-TX-TO-BLOCK"))
		if err != nil {
			return err
		}
		syncInfo, err := tx.CreateBucketIfNotExists([]byte("SYNC-INFO"))
		if err != nil {
			return err
		}

		// first store the compressed chunk of blocks
		// everythin else is referencing information
		chunkHash, _ := hex.DecodeString(blocks.Blocks[0].GetHash())
		err = chunks.Put(chunkHash, compressionBuf[:nCompressed])
		if err != nil {
			return err
		}

		for _, block := range blocks.Blocks {
			blockHash, _ := hex.DecodeString(block.GetHash())
			blockHeight := make([]byte, 8)
			binary.BigEndian.PutUint64(blockHeight, block.GetHeight())

			// eventually we'll need to figure out the winning
			// block and only commit that!
			err = height2hash.Put(blockHeight, blockHash)
			if err != nil {
				return err
			}

			for _, tx := range block.Txs {
				txHash, _ := hex.DecodeString(tx.GetHash())
				err = tx2hash.Put(txHash, blockHash)
				if err != nil {
					return err
				}
			}

			err = block2chunk.Put(blockHash, chunkHash)
			if err != nil {
				return err
			}

			err = syncInfo.Put([]byte("LastWrittenBlockHeight"), blockHeight)
			err = syncInfo.Put([]byte("LastWrittenBlockHash"), blockHash)
			if err != nil {
				return err
			}
		}
		return err
	})
}

func (odb *OverlineDB) deleteBlocksAfter(pruneChainTo uint64) error {
	for hash, block := range odb.incomingBlocks {
		if block.GetHeight() > pruneChainTo {
			delete(odb.incomingBlocks, hash)
		}
	}

	err := odb.db.Update(func(tx *bolt.Tx) error {
		blockChunks := tx.Bucket(OverlineBlockChunks)
		hash2chunk := tx.Bucket(OverlineBlockChunkMap)
		height2hash := tx.Bucket(OverlineHeightToHashMap)
		tx2Hash := tx.Bucket(OverlineTxToHashMap)
		syncInfo := tx.Bucket(SyncInfo)

		seekBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(seekBytes, pruneChainTo)

		currentChunk := make([]byte, 32)
		cHeights := height2hash.Cursor()

		// move whatever part of the chunk is below the specified height
		// back into the chainstate
		empty := make([]byte, 32)
		blockMap := make(map[string]*p2p_pb.BcBlock)
		heightKeysToDelete := make([][]byte, 0, 1000)
		for heightBytes, hashBytes := cHeights.Seek(seekBytes); heightBytes != nil; heightBytes, hashBytes = cHeights.Next() {
			chunkHash := hash2chunk.Get(hashBytes)
			decompressBlocks := false
			if bytes.Compare(currentChunk, empty) == 0 {
				// the current chunk starts out empty
				copy(currentChunk, chunkHash)
				decompressBlocks = true
			} else if bytes.Compare(currentChunk, chunkHash) != 0 {
				// we are done with the previous chunk and can delete it
				blockChunks.Delete(currentChunk)
				copy(currentChunk, chunkHash)
				decompressBlocks = true
			}
			// fill the block list if we need to
			if decompressBlocks {
				blockMap = make(map[string]*p2p_pb.BcBlock)
				chunk := blockChunks.Get(currentChunk)
				decompressionBuf := make([]byte, 10*len(chunk))
				nDecompressed, err := lz4.UncompressBlock(chunk, decompressionBuf)
				blocks := new(p2p_pb.BcBlocks)
				err = proto.Unmarshal(decompressionBuf[:nDecompressed], blocks)
				if err != nil {
					return err
				}
				for _, block := range blocks.Blocks {
					heightThisBlock := block.GetHeight()
					blockHeightBytes := make([]byte, 8)
					binary.BigEndian.PutUint64(blockHeightBytes, heightThisBlock)
					heightKeysToDelete = append(heightKeysToDelete, blockHeightBytes)
					blockMap[block.GetHash()] = block
				}
			}
			// if <= pruneChainTo put in incomingBlocks, delete from serialization map, delete all txs refs
			hashStr := hex.EncodeToString(hashBytes)
			block := blockMap[hashStr]
			if block.GetHeight() <= pruneChainTo {
				odb.incomingBlocks[hashStr] = block
			}
			for _, tx := range block.Txs {
				txHash, _ := hex.DecodeString(tx.GetHash())
				err := tx2Hash.Delete(txHash)
				if err != nil {
					return err
				}
			}
			hash2chunk.Delete(hashBytes)
			delete(blockMap, hashStr)
		}
		// reset sync data
		lastBlockHeight := binary.BigEndian.Uint64(heightKeysToDelete[0]) - 1
		lastBlockHeightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(lastBlockHeightBytes, lastBlockHeight)
		err := syncInfo.Put([]byte("LastWrittenBlockHeight"), lastBlockHeightBytes)
		if err != nil {
			return err
		}
		err = syncInfo.Put([]byte("LastWrittenBlockHash"), height2hash.Get(lastBlockHeightBytes))
		if err != nil {
			return err
		}
		// remove all touched heights from serialization index
		for _, heightKey := range heightKeysToDelete {
			height2hash.Delete(heightKey)
		}
		return nil
	})
	return err
}

func (odb *OverlineDB) GetSyncStartingBlock() *p2p_pb.BcBlock {
	return odb.syncStartingBlock
}
