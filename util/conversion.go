/*
 * Copyright (C) 2019 ING BANK N.V.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package util

import (
	"encoding/binary"
	"errors"
	"math/big"
)

/**
 * Returns the big.Int based on the passed byte-array assuming the byte-array contains
 * the two's-complement representation of this big.Int. The byte array will be in big-endian byte-order:
 * the most significant byte is in the zeroth element. The array will contain the minimum number of bytes
 * required to represent this BigInteger, including at least one sign bit, which is (ceil((this.bitLength() + 1)/8)).
 */
func FromByteArray(bytesIn []byte) (*big.Int, error) {

	const MINUS_ONE = -1

	if len(bytesIn) == 0 {
		err := errors.New("cannot convert empty array to big.Int")
		return nil, err
	}

	highestByte := bytesIn[0]
	isNegative := (highestByte & 128) != 0
	var convertedBytes []byte

	if isNegative {

		tmpInt := new(big.Int).SetBytes(bytesIn)
		tmpInt = tmpInt.Sub(tmpInt, big.NewInt(1))
		tmpBytes := tmpInt.Bytes()

		if tmpBytes[0] == 255 {
			convertedBytes = FlipBytes(tmpBytes)[1:]
		} else {
			convertedBytes = tmpBytes
			copy(convertedBytes, FlipBytes(tmpBytes))
		}
		tmp := new(big.Int).SetBytes(convertedBytes)
		return tmp.Mul(tmp, big.NewInt(MINUS_ONE)), nil
	} else {
		// if positive leave unchanged (additional 0-bytes will be ignored)
		return new(big.Int).SetBytes(bytesIn), nil
	}
}

func IntToBytes(i uint32) []byte {
	result := make([]byte, 4)
	binary.BigEndian.PutUint32(result, uint32(i))
	return result

}

func IntsToBytes(a []uint32) []byte {
	result := make([]byte, 4*len(a))
	for i, miscVal := range a {
		miscValBytes := IntToBytes(miscVal)
		for j := 0; j < 4; j++ {
			result[j+4*i] = miscValBytes[j]
		}
	}
	return result
}

func BytesToInt(a []byte) uint32 {
	return BytesToInts(a)[0]
}

func BytesToInts(a []byte) []uint32 {
	n := len(a) / 4
	result := make([]uint32, n)
	for j := 0; j < n; j++ {
		result[j] = binary.BigEndian.Uint32(a[4*j : 4*j+4])
	}
	return result
}

func ValsToPrefix(vals []uint32) []byte {
	result := make([]byte, 0)
	n := len(vals)
	for i := 0; i < n; i++ {
		partialPrefix := make([]byte, 4)
		binary.BigEndian.PutUint32(partialPrefix, uint32(vals[i]))
		result = append(result, partialPrefix...)
	}
	return result
}

func PrefixToVals(prefix []byte) []uint32 {
	result := make([]uint32, 0)
	n := len(prefix) / 4
	for i := 0; i < n; i++ {
		partialPrefix := make([]byte, 0)
		for j := 0; j < 4; j++ {
			partialPrefix = append(partialPrefix, prefix[i*4+j])
		}
		k := binary.BigEndian.Uint32(partialPrefix)
		result = append(result, k)
	}
	return result
}

/**
 * Returns a byte array containing the two's-complement representation of this big.Int.
 * The byte array will be in big-endian byte-order: the most significant byte is in the
 * zeroth element. The array will contain the minimum number of bytes required to represent
 * this BigInteger, including at least one sign bit, which is (ceil((this.bitLength() + 1)/8)).
 */
func ToByteArray(in *big.Int) []byte {

	isNegative := in.Cmp(new(big.Int).SetInt64(0)) < 0

	bytes := in.Bytes()
	length := len(bytes)

	if length == 0 {
		return []byte{0}
	}

	highestByte := bytes[0]
	var convertedBytes []byte

	if !isNegative {
		if (highestByte & 128) != 0 {

			convertedBytes = make([]byte, length+1)
			convertedBytes[0] = 0
			copy(convertedBytes[1:], bytes)
			return convertedBytes
		} else {
			return bytes
		}
	} else {
		if (highestByte & 128) != 0 {

			convertedBytes = make([]byte, length+1)
			convertedBytes[0] = 255
			copy(convertedBytes[1:], FlipBytes(bytes))
		} else {
			convertedBytes = FlipBytes(bytes)
		}

		convertedInt := new(big.Int).SetBytes(convertedBytes)
		convertedInt.Add(convertedInt, big.NewInt(1))
		return convertedInt.Bytes()
	}
}

/**
 * Flips all bytes in each of the array's elements.
 * Returns the flipped elements.
 */
func FlipBytes(bytesIn []byte) []byte {

	length := len(bytesIn)
	flippedBytes := make([]byte, length)

	for i := 0; i < length; i++ {
		flippedBytes[i] = bytesIn[i] ^ 255
	}
	return flippedBytes
}

func Flip(a []byte) []byte {
	n := len(a)
	b := make([]byte, n)
	for i, x := range a {
		b[n-i-1] = x
	}
	return b
}

func BigFromBase10(value string) *big.Int {
	i, _ := new(big.Int).SetString(value, 10)
	return i
}
