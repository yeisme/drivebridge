package service

import "testing"

func TestSecureRandomBytesRejectsInvalidSizeAndReturnsExactLength(t *testing.T) {
	if _, err := secureRandomBytes(0); err == nil {
		t.Fatal("zero-sized random request must fail")
	}
	b, err := secureRandomBytes(32)
	if err != nil || len(b) != 32 {
		t.Fatalf("secure random bytes: len=%d err=%v", len(b), err)
	}
}
