package service

import (
	"crypto/rand"
	"errors"
)

// secureRandomBytes is the single production boundary for generated keys,
// nonces and opaque cursor material. Partial reads are failures, even when the
// crypto/rand reader reports no error.
func secureRandomBytes(size int) ([]byte, error) {
	if size <= 0 {
		return nil, errors.New("random_size_invalid")
	}
	b := make([]byte, size)
	n, err := rand.Read(b)
	if err != nil || n != len(b) {
		return nil, errors.New("random_unavailable")
	}
	return b, nil
}
