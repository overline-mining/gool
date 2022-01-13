package genesis

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/overline-mining/gool/src/coin"
	"github.com/overline-mining/gool/src/olhash"
	p2p_pb "github.com/overline-mining/gool/src/protos"
	"github.com/overline-mining/gool/src/transactions"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/blake2b"
)

func createGenesisTransaction(miner string, balance *big.Int) (*p2p_pb.TransactionOutput, error) {
	genTxOutput := new(p2p_pb.TransactionOutput)
	hashedMiner := olhash.Blake2bl(olhash.Blake2bl(miner) + miner)
	outputScript, err := transactions.FromASM(fmt.Sprintf("OP_BLAKE2BLPRIV 0x%v OP_EQUALVERIFY OP_CHECKSIGNOPUBKEYVERIFY", hashedMiner), 0x01)
	if err != nil {
		return nil, err
	}
	// there is no need to figure out divisors or any of the baroque multi-precision math
	// we know exactly what we are building
	genTxOutput.Value = balance.Bytes()
	genTxOutput.Unit = []byte{1}
	genTxOutput.ScriptLength = uint32(len(outputScript))
	genTxOutput.OutputScript = outputScript
	return genTxOutput, nil
}

func BuildGenesisBlock(workDir string) (*p2p_pb.BcBlock, error) {
	gStartingBalancesPath := filepath.Join(workDir, "data", "genesis", "at_genesis_balances.csv")
	gblock := new(p2p_pb.BcBlock)

	balFile, err := os.Open(gStartingBalancesPath)
	if err != nil {
		return nil, err
	}
	defer balFile.Close()

	csvReader := csv.NewReader(balFile)
	csvReader.ReuseRecord = false
	csvReader.FieldsPerRecord = 2

	// create the genesis transaction
	var balanceStr string
	genTxs := make([]*p2p_pb.Transaction, 0, 1)
	genTx := new(p2p_pb.Transaction)
	// there are no utxos to gather for the genesis block
	genTx.Inputs = make([]*p2p_pb.TransactionInput, 0)
	genTx.NinCount = 0
	// other boilerplate
	//genTx.Nonce = MINER
	genTx.Version = 1
	genTx.Overline = "0"
	genTx.LockTime = coin.COINBASE_MATURITY
	genTxOutputs := make([]*p2p_pb.TransactionOutput, 0)
	unit, _ := new(big.Rat).SetString("1e18")
	for rec, err := csvReader.Read(); rec != nil; rec, err = csvReader.Read() {
		if len(rec[0]) != 42 || rec[0][:2] != "0x" {
			balanceStr += strings.Join(rec, ",")
			continue
		}
		if err != nil {
			return nil, err
		}
		balanceStr += strings.Join(rec, ",")
		balance, parsed := new(big.Int).SetString(rec[1], 10)
		if !parsed {
			return nil, errors.New(fmt.Sprintf("Unable to parse balance for %v: %v", rec[0], rec[1]))
		}
		balance.Mul(balance, unit.Num())
		theTXO, err := createGenesisTransaction(rec[0], balance)
		if err != nil {
			return nil, err
		}
		genTxOutputs = append(genTxOutputs, theTXO)
	}
	calcBalanceDigest := blake2b.Sum512([]byte(balanceStr))
	expBalanceDigest, err := hex.DecodeString(AT_BALANCE_DIGEST)
	if err != nil {
		return nil, err
	}
	if bytes.Compare(calcBalanceDigest[:], expBalanceDigest) != 0 {
		return nil, errors.New(fmt.Sprintf("Calculated genesis balance digest is incorrect:\n\t%v\n\t%v\n", hex.EncodeToString(calcBalanceDigest[:]), AT_BALANCE_DIGEST))
	}
	genTx.Outputs = genTxOutputs
	genTx.NoutCount = uint64(len(genTxOutputs))
	genTx.Hash = transactions.TxHash(genTx)
	genTxs = append(genTxs, genTx)
	gblock.Txs = genTxs
	gblock.TxCount = uint64(len(genTxs))

	gblock.Hash = HASH
	gblock.PreviousHash = PREVIOUS_HASH
	gblock.Version = VERSION
	gblock.SchemaVersion = SCHEMA_VERSION
	gblock.Height = HEIGHT
	gblock.Miner = MINER
	gblock.Difficulty = DIFFICULTY
	gblock.Timestamp = TIMESTAMP
	gblock.MerkleRoot = MERKLE_ROOT
	gblock.ChainRoot = CHAIN_ROOT
	gblock.Distance = DISTANCE
	gblock.TotalDistance = TOTAL_DISTANCE
	gblock.Nonce = NONCE
	gblock.NrgGrant = NRG_GRANT
	gblock.Twn = TWN
	gblock.EmblemWeight = EMBLEM_WEIGHT
	gblock.EmblemChainFingerprintRoot = EMBLEM_CHAIN_FINGERPRINT_ROOT
	gblock.EmblemChainAddress = EMBLEM_CHAIN_ADDRESS
	gblock.TxFeeBase = TX_FEE_BASE
	gblock.TxDistanceSumLimit = TX_DISTANCE_SUM_LIMIT
	gblock.BlockchainFingerprintsRoot = BLOCKCHAIN_FINGERPRINTS_ROOT
	gblock.BlockchainHeaders = new(p2p_pb.BlockchainHeaders)

	genesisHash := olhash.Blake2bl(
		gblock.GetMiner() +
			gblock.GetMerkleRoot() +
			gblock.GetBlockchainFingerprintsRoot() +
			gblock.GetEmblemChainFingerprintRoot() +
			gblock.GetDifficulty())

	if genesisHash != gblock.GetHash() {
		return nil, errors.New(fmt.Sprintf("Calculated genesis hash %v != %v", genesisHash, HASH))
	}

	return gblock, nil
}
