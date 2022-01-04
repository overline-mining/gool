package olhash

import (
	"encoding/hex"
	"math"
	"math/big"
	"strconv"

	"golang.org/x/crypto/blake2b"
)

func Blake2bl(data string) string {
	return hex.EncodeToString(Blake2blFromBytes([]byte(data)))
}

func Blake2blFromBytes(data []byte) []byte {
	hash := blake2b.Sum512(data)
	return hash[32:]
}

func CalcDistance(work []byte, soln []byte) uint64 {
	acc := float64(0.0)
	num := float64(0.0)
	den := float64(0.0)
	norm_w := float64(0.0)
	norm_s := float64(0.0)

	for i := 0; i < len(work)/32; i++ {
		num = 0.0
		den = 0.0
		norm_w = 0.0
		norm_s = 0.0
		for j := 0; j < 32; j++ {
			w := float64(work[32*(1-i)+j])
			s := float64(soln[32*i+j])
			num += w * s
			norm_w += w * w
			norm_s += s * s
		}
		den = math.Sqrt(norm_w) * math.Sqrt(norm_s)
		acc += (1.0 - num/den)
	}
	return uint64(acc * float64(uint64(1000000000000000)))
}

func eval(work []byte, miner_key []byte, merkle_root []byte,
	nonce uint64, timestamp int64) uint64 {
	nonce_bytes := []byte(strconv.FormatUint(nonce, 10))
	timestamp_bytes := []byte(strconv.FormatInt(timestamp, 10))

	nonce_hash_str := hex.EncodeToString(Blake2blFromBytes(nonce_bytes))
	nonce_hash := []byte(nonce_hash_str)

	tohash := append(miner_key, merkle_root...)
	tohash = append(tohash, nonce_hash...)
	tohash = append(tohash, timestamp_bytes...)

	guess := []byte(hex.EncodeToString(Blake2blFromBytes(tohash)))

	return CalcDistance(work, guess)
}

func Verify(Difficulty *big.Int, Work string, MinerKey string, MerkleRoot string, Nonce uint64, Timestamp int64) bool {
	work := []byte(Work)
	miner_key := []byte(MinerKey)
	merkle_root := []byte(MerkleRoot)
	nonce := Nonce
	ts := Timestamp

	calc_dist := eval(work, miner_key, merkle_root, nonce, ts)

	return calc_dist >= Difficulty.Uint64()
}
