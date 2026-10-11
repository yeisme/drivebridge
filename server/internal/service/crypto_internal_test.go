package service

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"
)

func aeadFor(t *testing.T, key []byte) cipher.AEAD {
	t.Helper()
	a, e := contentAEAD(key)
	if e != nil {
		t.Fatal(e)
	}
	return a
}

func sealStream(t *testing.T, key []byte, plain []byte, partSize int64) []byte {
	t.Helper()
	aead := aeadFor(t, key)
	nonce := make([]byte, 12)
	rand.Read(nonce)
	var out bytes.Buffer
	count := (int64(len(plain)) + partSize - 1) / partSize
	for index := 0; int64(index)*partSize < int64(len(plain)); index++ {
		lo := int64(index) * partSize
		hi := min(lo+partSize, int64(len(plain)))
		startChunk := lo / contentChunk
		finalChunk := int64(-1)
		if int64(index) == count-1 {
			finalChunk = (int64(len(plain)) - 1) / contentChunk
		}
		sealer := newContentSealer(aead, nonce, startChunk, finalChunk, int64(len(plain)), &out)
		if _, e := sealer.Write(plain[lo:hi]); e != nil {
			t.Fatal(e)
		}
		if e := sealer.Close(); e != nil {
			t.Fatal(e)
		}
		if sealer.written != contentFrameSize(hi-lo) {
			t.Fatalf("part %d frame size %d, want %d", index, sealer.written, contentFrameSize(hi-lo))
		}
	}
	header := append([]byte(contentMagic), byte(1))
	header = append(header, nonce...)
	return append(header, out.Bytes()...)
}

func mustOpen(t *testing.T, key []byte, blob []byte, total int64) io.Reader {
	t.Helper()
	opener, e := newContentOpener(aeadFor(t, key), bytes.NewReader(blob), total)
	if e != nil {
		t.Fatal(e)
	}
	return opener
}

func TestEnvelopeStreamRoundtripSizesAndTamper(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	for _, size := range []int64{1, 100, 65535, 65536, 65537, 1 << 20, 9<<20 + 12345, 12 << 20} {
		plain := make([]byte, size)
		rand.Read(plain)
		for _, partSize := range []int64{1 << 20, 8 << 20} {
			blob := sealStream(t, key, plain, partSize)
			plain2, e := io.ReadAll(mustOpen(t, key, blob, size))
			if e != nil || !bytes.Equal(plain2, plain) {
				t.Fatalf("size=%d part=%d roundtrip differs: %v", size, partSize, e)
			}
			if _, e := io.ReadAll(mustOpen(t, key, blob[:len(blob)-7], size)); e == nil {
				t.Fatal("truncated stream accepted")
			}
			if _, e := io.ReadAll(mustOpen(t, key, append(append([]byte{}, blob...), 0), size)); e == nil {
				t.Fatal("extended stream accepted")
			}
			corrupt := append([]byte{}, blob...)
			corrupt[len(corrupt)-3] ^= 0x40
			if _, e := io.ReadAll(mustOpen(t, key, corrupt, size)); e == nil {
				t.Fatal("tampered stream accepted")
			}
			if _, e := io.ReadAll(mustOpen(t, key, blob, size+1)); e == nil {
				t.Fatal("wrong total accepted")
			}
		}
	}
}
