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

func l2norm(vec []byte) float64 {
	t := float64(0)
	s := float64(1)
	r := float64(0)
	val := float64(0)
	for _, abyte := range vec {
		val = float64(abyte)
		if val > 0 {
			if val > t {
				r = t / val
				s = 1 + float64(s*r*r)
				t = val
			} else {
				r = val / t
				s = s + float64(r*r)
			}
		}
	}
	return t * math.Sqrt(s)
}

func CalcDistance(work []byte, soln []byte) uint64 {
	acc := float64(0)
	num := float64(0)

	for i := 0; i < len(work)/32; i++ {
		num = float64(0)
		norm_w := l2norm(work[32*(1-i) : 32*(1-i)+32])
		norm_s := l2norm(soln[32*i : 32*i+32])
		for j := 0; j < 32; j++ {
			w := float64(work[32*(1-i)+j])
			s := float64(soln[32*i+j])
			num += float64(s * w)
		}
		sim := 1 - float64(num/(norm_s*norm_w))
		acc = acc + sim
	}
	return uint64(math.Floor(acc * 1000000000000000))
}

func Eval(work, miner_key, merkle_root, nonce []byte, timestamp uint64) uint64 {
	timestamp_bytes := []byte(strconv.FormatUint(timestamp, 10))

	nonce_hash := []byte(hex.EncodeToString(Blake2blFromBytes(nonce)))

	tohash := append(miner_key, merkle_root...)
	tohash = append(tohash, nonce_hash...)
	tohash = append(tohash, timestamp_bytes...)

	guess := []byte(hex.EncodeToString(Blake2blFromBytes(tohash)))

	return CalcDistance(work, guess)
}

func EvalString(work, miner_key, merkle_root, nonce string, timestamp uint64) uint64 {

	return Eval([]byte(work), []byte(miner_key), []byte(merkle_root), []byte(nonce), timestamp)
}

func Verify(Difficulty *big.Int, Work, MinerKey, MerkleRoot, Nonce string, Timestamp uint64) bool {
	work := []byte(Work)
	miner_key := []byte(MinerKey)
	merkle_root := []byte(MerkleRoot)
	nonce := []byte(Nonce)
	ts := Timestamp

	calc_dist := Eval(work, miner_key, merkle_root, nonce, ts)

	return calc_dist >= Difficulty.Uint64()
}
