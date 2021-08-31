package gonanoid

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
)

var defaultAlphabet = []rune("_-0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
const (
	defaultSize     = 21
	defaultMaskSize = 5
)

// Generator function
type Generator func([]byte) (int, error)

// BytesGenerator is the default bytes generator
var BytesGenerator Generator = rand.Read

func initMasks(params ...int) []uint {
	var size int
	if len(params) == 0 {
		size = defaultMaskSize
	} else {
		size = params[0]
	}
	masks := make([]uint, size)
	for i := 0; i < size; i++ {
		shift := 3 + i
		masks[i] = (2 << uint(shift)) - 1
	}
	return masks
}

func getMask(alphabet []rune, masks []uint) int {
	for i := 0; i < len(masks); i++ {
		curr := int(masks[i])
		if curr >= len(alphabet)-1 {
			return curr
		}
	}
	return 0
}

// Generate is a low-level function to change alphabet and ID size.
func Generate(rawAlphabet string, size int) (string, error) {
	alphabet := []rune(rawAlphabet)

	if len(alphabet) == 0 || len(alphabet) > 255 {
		return "", fmt.Errorf("alphabet must not empty and contain no more than 255 chars. Current len is %d", len(alphabet))
	}
	if size <= 0 {
		return "", fmt.Errorf("size must be positive integer")
	}

	masks := initMasks(size)
	mask := getMask(alphabet, masks)
	ceilArg := 1.6 * float64(mask*size) / float64(len(alphabet))
	step := int(math.Ceil(ceilArg))

	id := make([]rune, size)
	bytes := make([]byte, step)
	for j := 0; ; {
		_, err := BytesGenerator(bytes)
		if err != nil {
			return "", err
		}
		for i := 0; i < step; i++ {
			currByte := bytes[i] & byte(mask)
			if currByte < byte(len(alphabet)) {
				id[j] = alphabet[currByte]
				j++
				if j == size {
					return string(id[:size]), nil
				}
			}
		}
	}
}

// Nanoid generates secure URL-friendly unique ID.
func Nanoid(param ...int) (string, error) {
	var size int
	switch {
	case len(param) == 0:
		size = defaultSize
	case len(param) == 1:
		size = param[0]
		if size < 0 {
			return "", errors.New("negative id length")
		}
	default:
		return "", errors.New("unexpected parameter")
	}
	bytes := make([]byte, size)
	_, err := BytesGenerator(bytes)
	if err != nil {
		return "", err
	}
	id := make([]rune, size)
	for i := 0; i < size; i++ {
		id[i] = defaultAlphabet[bytes[i]&63]
	}
	return string(id[:size]), nil
}

// ID provides more golang idiomatic interface for generating IDs.
// Calling ID is shorter yet still clear `gonanoid.ID(20)` and it requires the lengths parameter by default.
func ID(l int) (string, error) {
	return Nanoid(l)
}

// MustID is the same as ID but panics on error.
func MustID(l int) string {
	id, err := Nanoid(l)
	if err != nil {
		panic(err)
	}
	return id
}

// MustGenerate is the same as Generate but panics on error.
func MustGenerate(rawAlphabet string, size int) string {
	id, err := Generate(rawAlphabet, size)
	if err != nil {
		panic(err)
	}
	return id
}
