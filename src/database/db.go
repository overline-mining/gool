package database

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
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
)

var ChainstateBlocks = []byte("CHAINSTATE-BLOCKS")
var ChainstateTxs = []byte("CHAINSTATE-TXS")
var ChainstateMTxs = []byte("CHAINSTATE-MTXS")

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

type OverlineDB struct {
	Config               OverlineDBConfig
	db                   *bolt.DB
	mu                   sync.Mutex
	txMu                 sync.Mutex
	tipOfSerializedChain *p2p_pb.BcBlock   // the highest, main-chain serialized block
	highestBlock         *p2p_pb.BcBlock   // the highest block with the most distance
	toSerialize          []*p2p_pb.BcBlock // sorted ascending in block height
	incomingBlocks       map[string]*p2p_pb.BcBlock
	txMemPool            map[string]*p2p_pb.Transaction
	mtxMemPool           map[string]*p2p_pb.MarkedTransaction
}

func (odb *OverlineDB) Open(filepath string) error {
	var err error
	odb.db, err = bolt.Open(filepath, 0600, nil)
	odb.mu.Lock()
	odb.toSerialize = make([]*p2p_pb.BcBlock, 0, 10*odb.Config.AncientChunkSize)
	odb.incomingBlocks = make(map[string]*p2p_pb.BcBlock)
	odb.txMu.Lock()
	odb.txMemPool = make(map[string]*p2p_pb.Transaction)
	odb.mtxMemPool = make(map[string]*p2p_pb.MarkedTransaction)
	odb.txMu.Unlock()

	err = odb.db.View(func(tx *bolt.Tx) error {
		blocks := tx.Bucket(ChainstateBlocks)
		txs := tx.Bucket(ChainstateTxs)
		mtxs := tx.Bucket(ChainstateMTxs)
		if blocks == nil {
			return errors.New("Uninitialized blockchain file!")
		}
		cblocks := blocks.Cursor()

		for hash, serblock := cblocks.First(); hash != nil; hash, serblock = cblocks.Next() {
			strhash := hex.EncodeToString(hash)
			block := new(p2p_pb.BcBlock)
			err := proto.Unmarshal(serblock, block)
			isValid, _ := validation.IsValidBlock(block)
			if err == nil && isValid {
				odb.incomingBlocks[strhash] = block
			}
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
	}
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

func (odb *OverlineDB) addBlockUnsafe(block *p2p_pb.BcBlock) {
	if _, ok := odb.incomingBlocks[block.GetHash()]; !ok {
		odb.incomingBlocks[block.GetHash()] = block
		blockDist, _ := new(big.Int).SetString(block.GetTotalDistance(), 10)
		highestDist, _ := new(big.Int).SetString(odb.highestBlock.GetTotalDistance(), 10)
		if block.GetPreviousHash() == odb.highestBlock.GetHash() &&
			block.GetHeight() == odb.highestBlock.GetHeight()+1 {
			odb.highestBlock = block
		} else if block.GetPreviousHash() == odb.highestBlock.GetPreviousHash() &&
			blockDist.Cmp(highestDist) > 0 {
			odb.highestBlock = block
		}
	} else {
		zap.S().Debugf("Block %v:%v already seen.", block.GetHeight(), common.BriefHash(block.GetHash()))
	}
}

func (odb *OverlineDB) AddBlock(block *p2p_pb.BcBlock) error {
	zap.S().Debug("AddBlock -> entered function!")
	isValid, err := validation.IsValidBlock(block)
	if !isValid {
		return err
	}
	zap.S().Debug("AddBlock -> Acquiring lock!")
	odb.mu.Lock()
	zap.S().Debug("AddBlock -> acquired lock!")
	odb.addBlockUnsafe(block)
	odb.mu.Unlock()
	zap.S().Debug("AddBlock -> released lock!")
	return nil
}

func (odb *OverlineDB) AddBlockRange(brange *p2p_pb.BcBlocks) error {
	for _, block := range brange.Blocks {
		isValid, err := validation.IsValidBlock(block)
		if !isValid {
			return err
		}
		odb.addBlockUnsafe(block)
	}
	return nil
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

func (odb *OverlineDB) FlushToDisk() {
	odb.mu.Lock()
	odb.txMu.Lock()
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
				for hash, newBlock := range odb.incomingBlocks {
					odb.toSerialize = append(odb.toSerialize, newBlock)
					delete(odb.incomingBlocks, hash)
				}
				sort.SliceStable(odb.toSerialize, func(i, j int) bool {
					if odb.toSerialize[i].GetHeight() == odb.toSerialize[j].GetHeight() {
						iDist, _ := new(big.Int).SetString(odb.toSerialize[i].GetTotalDistance(), 10)
						jDist, _ := new(big.Int).SetString(odb.toSerialize[j].GetTotalDistance(), 10)
						compare := iDist.Cmp(jDist)
						if compare == 0 {
							return odb.toSerialize[i].GetTimestamp() < odb.toSerialize[j].GetTimestamp()
						}
						return compare < 0
					}
					return odb.toSerialize[i].GetHeight() < odb.toSerialize[j].GetHeight()
				})
				if len(odb.toSerialize) > odb.Config.AncientChunkSize {
					odb.serializeBlocks(odb.toSerialize[:odb.Config.AncientChunkSize])
					odb.toSerialize = odb.toSerialize[odb.Config.AncientChunkSize:]
				}
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
