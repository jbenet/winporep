package winporep

import (
	"crypto/sha256"
)

var HashCounter = 0

func Hash(inplace []byte, vals ...[]byte) []byte {
	h := sha256.New()
	for _, v := range vals {
		HashCounter++
		h.Write(v)
	}
	return h.Sum(inplace)
}

// n needs to be smaller or equal than the length of a and b.
func safeXORBytes(dst, a, b []byte, n int) {
	for i := 0; i < n; i++ {
		dst[i] = a[i] ^ b[i]
	}
}

func XOR(dst []byte, vals ...[]byte) {
	for _, v := range vals {
		safeXORBytes(dst, dst, v, len(v))
	}
}
