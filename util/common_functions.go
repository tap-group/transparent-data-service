package util

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"io"
	"math"
	"math/big"
	"os"
	"strconv"

	"golang.org/x/crypto/sha3"
)

// type Snode struct {
// 	value     int
// 	rowID     int
// 	enc2      int      // NTL::ZZ_p enc2;
// 	encry     []byte   // unsigned char encry[255];
// 	g1_digest bn256.G1 // bn::Ec1 g1_digest;
// 	g2_digest bn256.G2 // bn::Ec2 g2_digest;
// 	hash      []byte   // unsigned char hash[SHA256_DIGEST_LENGTH];
// 	right     *Snode   // snode *right;
// 	up        *Snode   // snode *up;
// 	down      *Snode   // snode *down;
// 	right0    *Snode   // snode *right0;
// }

// type Skiplist struct {
// 	header *Snode
// }

const hashSize = 32

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func MakePrefixFromKey(key []byte) []byte {
	return ConvertBytesToBits(Hash(key))
}

func PowInt(x, y int) int {
	return int(math.Pow(float64(x), float64(y)))
}

func ValMoments(x, k int) []int {
	result := make([]int, k)
	for i := 0; i < k; i++ {
		result[i] = PowInt(x, i+1)
	}
	return result
}

func AllValMoments(x []int, k int) [][]int {
	n := len(x)
	result := make([][]int, n)
	for i := 0; i < n; i++ {
		result[i] = ValMoments(x[i], k)
	}
	return result
}

// Hash takes in []byte inputs and outputs a hash value
func Hash(ms ...[]byte) []byte {
	h := sha3.NewShake128() // TODO: do we want to change the function used?
	//NOTE: Google KT uses sha256.Sum256 -- Yuncong

	for _, m := range ms {
		h.Write(m)
	}
	ret := make([]byte, hashSize)
	h.Read(ret)

	return ret
}

// Hash takes in []byte inputs and outputs a hash value
func Hash32(ms ...[]byte) [32]byte {
	h := sha3.NewShake128() // TODO: do we want to change the function used?
	//NOTE: Google KT uses sha256.Sum256 -- Yuncong

	for _, m := range ms {
		h.Write(m)
	}
	ret := make([]byte, hashSize)
	h.Read(ret)

	return sha256.Sum256(ret)
}

// func HashCommitmentAndCount(commitment []byte, count int) [32]byte {
// 	lHash := make([]byte, 4)
// 	binary.LittleEndian.PutUint32(lHash, uint32(count))
// 	return Hash32(append(commitment, lHash...))
// }

func NilHash() [32]byte {
	return Hash32(nil)
}

func DoubleNilHash() []byte {
	nilHash := NilHash()
	return append(nilHash[:], nilHash[:]...)
}

// func NilHashCommitmentAndCount(commitment []byte, count int) [32]byte {
// 	commHash := HashCommitmentAndCount(commitment, count)
// 	return Hash32(append(DoubleNilHash(), commHash[:]...))
// }

func HashCommitmentsAndCount(commitments [][]byte, count int) [32]byte {
	if len(commitments) == 0 {
		return NilHash()
	}
	countHash := make([]byte, 4)
	binary.LittleEndian.PutUint32(countHash, uint32(count))
	commHash := make([]byte, 0)
	for i := 1; i < len(commitments); i++ {
		commHash = append(commHash, commitments[i]...)
	}
	return Hash32(append(commHash, countHash...))
}

func IdHash(uid, time int) [32]byte {
	uidBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(uidBytes, uint32(uid))
	timeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(timeBytes, uint32(time))
	return Hash32(uidBytes, timeBytes)
}

func ConvertBytesToBits(byteArray []byte) []byte {
	res := []byte{}

	for _, byt := range byteArray {
		for i := 0; i < 8; i++ {
			if ((1 << (7 - i)) & byt) == 0 {
				res = append(res, 0)
			} else {
				res = append(res, 1)
			}
		}
	}
	return res
}

func Decompose(x *big.Int, u int64, l int64) ([]int64, error) {
	var (
		result []int64
		i      int64
	)
	result = make([]int64, l)
	i = 0
	for i < l {
		result[i] = Mod(x, new(big.Int).SetInt64(u)).Int64()
		x = new(big.Int).Div(x, new(big.Int).SetInt64(u))
		i = i + 1
	}
	return result, nil
}

func ReadCsvFileEntry(filename string, idx int) []uint32 {
	file, err := os.Open(filename)
	Check(err)
	r := csv.NewReader(file)
	result := make([]string, 0)
	n := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		} else {
			Check(err)
		}
		if n == idx {
			result = record
		}
		n++
	}

	m := len(result)
	if m > 0 {
		intResult := make([]uint32, m)
		for i := 0; i < m; i++ {
			j, err := strconv.Atoi(result[i])
			Check(err)
			intResult[i] = uint32(j)
		}
		return intResult
	}

	return nil
}

// //bilinear accumulator functions
// func compute_digest_pub(array []int, g1 bn256.G1) bn256.G1 {
// 	digest := g1.ScalarBaseMult(big.NewInt(int64(0)))
// 	if len(array) == 0 {
// 		return *digest
// 	}

// 	// ZZ_pX f,poly;
// 	// poly=ZZ_pX(INIT_MONO,array.size());
// 	// vec_ZZ_p c;
// 	// c.SetLength(array.size());
// 	// for(int i=0;i<array.size();i++)
// 	// 	c[i] = conv<ZZ_p>(-array[i]);

// 	// BuildFromRoots(poly,c);

// 	// for(int i=0;i<array.size()+1;i++){

// 	// 	const mie::Vuint temp(zToString(poly[i]));
// 	// 	digest = digest+pubs_g1[i]*temp;
// 	// }
// 	return *digest
// }
