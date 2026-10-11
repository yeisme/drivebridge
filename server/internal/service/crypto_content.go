package service

// Content envelope encryption: per-space data keys (DEK) wrapped by an
// instance key-encryption key (KEK); object bytes are stored as framed
// AES-GCM ciphertext while metadata keeps the plaintext SHA-256.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"

	"gorm.io/gorm"
)

const (
	contentMagic = "DBE1"
	contentChunk = 64 << 10
	// contentHeaderLen = magic(4) + key version(1) + object nonce(12).
	contentHeaderLen = len(contentMagic) + 1 + 12
	kekInfo          = "drivebridge-kek-v1"
	backupKeyInfo    = "drivebridge-backup-v1"
	maxKeyVersion    = 255
)

// contentFrameSize returns the deterministic ciphertext length (object header
// excluded) of a chunk-aligned plaintext range of s bytes.
func contentFrameSize(s int64) int64 {
	if s == 0 {
		return 4 + 16
	}
	size := (s / contentChunk) * (4 + contentChunk + 16)
	if r := s % contentChunk; r > 0 {
		size += 4 + r + 16
	}
	return size
}

func chunkNonce(base []byte, index uint64) []byte {
	var n [12]byte
	copy(n[:], base)
	var idx [8]byte
	binary.BigEndian.PutUint64(idx[:], index)
	for i := 0; i < 8; i++ {
		n[4+i] ^= idx[i]
	}
	return n[:]
}

func chunkAAD(index uint64, final bool, total int64) []byte {
	aad := make([]byte, 0, 16)
	var idx [8]byte
	binary.BigEndian.PutUint64(idx[:], index)
	aad = append(aad, idx[:]...)
	if final {
		var t [8]byte
		binary.BigEndian.PutUint64(t[:], uint64(total))
		aad = append(aad, t[:]...)
	}
	return aad
}

// contentSealer writes framed ciphertext for one staged part. startChunk is
// the object-global index of the first chunk; finalChunk (or -1) marks the
// chunk that carries the total-length AAD.
type contentSealer struct {
	aead       cipher.AEAD
	base       []byte
	next       uint64
	finalChunk int64
	total      int64
	buf        []byte
	dst        io.Writer
	written    int64
}

func newContentSealer(aead cipher.AEAD, base []byte, startChunk, finalChunk, total int64, dst io.Writer) *contentSealer {
	return &contentSealer{aead: aead, base: base, next: uint64(startChunk), finalChunk: finalChunk, total: total, buf: make([]byte, 0, contentChunk), dst: dst}
}
func (c *contentSealer) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		space := int(contentChunk) - len(c.buf)
		n := min(space, len(p))
		c.buf = append(c.buf, p[:n]...)
		p = p[n:]
		total += n
		if len(c.buf) == contentChunk {
			if e := c.flush(); e != nil {
				return total, e
			}
		}
	}
	return total, nil
}
func (c *contentSealer) flush() error {
	isFinal := int64(c.next) == c.finalChunk
	ct := c.aead.Seal(nil, chunkNonce(c.base, c.next), c.buf, chunkAAD(c.next, isFinal, c.total))
	var head [4]byte
	binary.BigEndian.PutUint32(head[:], uint32(len(ct)))
	if _, e := c.dst.Write(head[:]); e != nil {
		return e
	}
	if _, e := c.dst.Write(ct); e != nil {
		return e
	}
	c.written += int64(4 + len(ct))
	c.next++
	c.buf = c.buf[:0]
	return nil
}
func (c *contentSealer) Close() error {
	if len(c.buf) > 0 || int64(c.next) == c.finalChunk {
		return c.flush()
	}
	return nil
}

// contentOpener streams decrypted plaintext from an object body. total is the
// expected plaintext length from metadata; every frame is authenticated
// against its chunk index and, for the final chunk, the total length, so
// truncation, reordering and extension all fail closed.
type contentOpener struct {
	aead    cipher.AEAD
	src     io.Reader
	base    []byte
	total   int64
	read    int64
	chunks  uint64
	pending []byte
	eof     bool
}

func newContentOpener(aead cipher.AEAD, src io.Reader, total int64) (*contentOpener, error) {
	header := make([]byte, contentHeaderLen)
	if _, e := io.ReadFull(src, header); e != nil {
		return nil, code("content_mismatch")
	}
	if string(header[:len(contentMagic)]) != contentMagic || header[len(contentMagic)] == 0 {
		return nil, code("content_mismatch")
	}
	return &contentOpener{aead: aead, src: src, base: header[len(contentMagic)+1:], total: total}, nil
}
func (o *contentOpener) Read(p []byte) (int, error) {
	for len(o.pending) == 0 {
		if o.eof {
			// One trailing probe distinguishes a clean end from smuggled
			// extra bytes after the final frame.
			var one [1]byte
			if n, _ := o.src.Read(one[:]); n != 0 {
				return 0, code("content_mismatch")
			}
			return 0, io.EOF
		}
		if o.read >= o.total && !(o.total == 0 && o.chunks == 0) {
			// The stream delivered every expected byte without the
			// total-length binding of a final frame.
			return 0, code("content_mismatch")
		}
		var head [4]byte
		if _, e := io.ReadFull(o.src, head[:]); e != nil {
			return 0, code("content_mismatch")
		}
		n := binary.BigEndian.Uint32(head[:])
		if n < 16 || n > contentChunk+16 {
			return 0, code("content_mismatch")
		}
		ct := make([]byte, n)
		if _, e := io.ReadFull(o.src, ct); e != nil {
			return 0, code("content_mismatch")
		}
		final := o.read+int64(n-16) == o.total
		if !final && o.read+int64(n-16) > o.total {
			return 0, code("content_mismatch")
		}
		plain, e := o.aead.Open(nil, chunkNonce(o.base, o.chunks), ct, chunkAAD(o.chunks, final, o.total))
		if e != nil {
			return 0, code("content_mismatch")
		}
		o.read += int64(len(plain))
		o.chunks++
		o.pending = plain
		if final {
			o.eof = true
		}
	}
	n := copy(p, o.pending)
	o.pending = o.pending[n:]
	return n, nil
}

// verifyReader fails the EOF boundary when the decrypted plaintext digest
// does not match the metadata digest.
type verifyReader struct {
	r    *contentOpener
	h    hash.Hash
	want string
	end  bool
}

func newVerifyReader(r *contentOpener, want string) *verifyReader {
	return &verifyReader{r: r, h: sha256.New(), want: want}
}
func (v *verifyReader) Read(p []byte) (int, error) {
	n, e := v.r.Read(p)
	if n > 0 {
		v.h.Write(p[:n])
	}
	if e == io.EOF && !v.end {
		v.end = true
		if hex.EncodeToString(v.h.Sum(nil)) != v.want {
			return n, code("content_mismatch")
		}
	}
	return n, e
}
func (v *verifyReader) Close() error {
	if c, ok := v.r.src.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
func (v *verifyReader) Verify() error {
	var probe [1]byte
	_, e := v.Read(probe[:])
	// Read reports EOF only after the digest matched; data past the expected
	// end is a mismatch and any other error passes through.
	if e == io.EOF {
		return nil
	}
	if e == nil {
		return code("content_mismatch")
	}
	return e
}

func contentAEAD(key []byte) (cipher.AEAD, error) {
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}

// kekFor derives the key-encryption key for a wrap version. Version 1 derives
// directly from the instance key; rotated versions unwrap their random key
// material (stored wrapped under version 1) so restores stay reachable from
// instance.key alone.
func (s *Service) kekFor(version int) (cipher.AEAD, error) {
	if version < 1 || version > maxKeyVersion {
		return nil, code("encryption_unavailable")
	}
	if version == 1 {
		key, e := deriveKey(s.key, kekInfo)
		if e != nil {
			return nil, code("encryption_unavailable")
		}
		return contentAEAD(key)
	}
	var row KeyMaterial
	if s.DB.First(&row, "version = ?", version).Error != nil || len(row.Wrapped) < 12+32+16 {
		return nil, code("encryption_unavailable")
	}
	root, e := deriveKey(s.key, kekInfo)
	if e != nil {
		return nil, code("encryption_unavailable")
	}
	rootAEAD, e := contentAEAD(root)
	if e != nil {
		return nil, code("encryption_unavailable")
	}
	plain, e := rootAEAD.Open(nil, row.Wrapped[:12], row.Wrapped[12:], []byte(fmt.Sprintf("key_rotation:%d", version)))
	if e != nil || len(plain) != 32 {
		return nil, code("encryption_unavailable")
	}
	return contentAEAD(plain)
}

func deriveKey(secret []byte, info string) ([]byte, error) {
	return hkdf.Key(sha256.New, secret, nil, info, 32)
}

func (s *Service) currentKEKVersion() int {
	var max int
	s.DB.Model(&KeyMaterial{}).Select("COALESCE(MAX(version), 0)").Scan(&max)
	if max < 1 {
		return 1
	}
	return max
}

// RotateKEK re-wraps every encrypted space's DEK under a fresh key-encryption
// key without touching stored object bytes.
func (s *Service) RotateKEK() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var spaces []Space
	if e := s.DB.Where("encrypted = ?", true).Find(&spaces).Error; e != nil {
		return nil, code("database_unavailable")
	}
	next := s.currentKEKVersion() + 1
	if next > maxKeyVersion {
		return nil, code("key_version_exhausted")
	}
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		return nil, code("encryption_unavailable")
	}
	root, e := deriveKey(s.key, kekInfo)
	if e != nil {
		return nil, code("encryption_unavailable")
	}
	rootAEAD, e := contentAEAD(root)
	if e != nil {
		return nil, code("encryption_unavailable")
	}
	nonce := make([]byte, 12)
	rand.Read(nonce)
	wrapped := append(append([]byte{}, nonce...), rootAEAD.Seal(nil, nonce, raw, []byte(fmt.Sprintf("key_rotation:%d", next)))...)
	nextAEAD, e := contentAEAD(raw)
	if e != nil {
		return nil, code("encryption_unavailable")
	}
	rewrapped := 0
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&KeyMaterial{Version: next, Wrapped: wrapped}).Error; e != nil {
			return code("database_unavailable")
		}
		for _, sp := range spaces {
			dek, e := s.spaceDEK(sp)
			if e != nil {
				return e
			}
			blob, e := wrapDEK(nextAEAD, next, sp.ID, dek)
			if e != nil {
				return e
			}
			if e := tx.Model(&Space{}).Where("id = ?", sp.ID).Update("wrapped_dek", blob).Error; e != nil {
				return code("database_unavailable")
			}
			rewrapped++
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return map[string]any{"key_version": next, "spaces_rewrapped": rewrapped, "objects_rewritten": 0}, nil
}

func wrapDEK(kek cipher.AEAD, version int, spaceID string, dek []byte) ([]byte, error) {
	nonce := make([]byte, 12)
	rand.Read(nonce)
	blob := append([]byte{byte(version)}, nonce...)
	return append(blob, kek.Seal(nil, nonce, dek, []byte("space_dek:"+spaceID))...), nil
}

// newSpaceDEK mints a wrapped DEK for a freshly created encrypted space.
func (s *Service) newSpaceDEK(spaceID string) ([]byte, error) {
	version := s.currentKEKVersion()
	kek, e := s.kekFor(version)
	if e != nil {
		return nil, e
	}
	dek := make([]byte, 32)
	if _, e = rand.Read(dek); e != nil {
		return nil, code("encryption_unavailable")
	}
	return wrapDEK(kek, version, spaceID, dek)
}

// spaceDEK unwraps a space's data key; the plaintext key never leaves memory.
func (s *Service) spaceDEK(sp Space) ([]byte, error) {
	if !sp.Encrypted || len(sp.WrappedDEK) < 1+12+32+16 {
		return nil, code("encryption_unavailable")
	}
	version := int(sp.WrappedDEK[0])
	if version < 1 || version > maxKeyVersion {
		return nil, code("encryption_unavailable")
	}
	kek, e := s.kekFor(version)
	if e != nil {
		return nil, e
	}
	dek, e := kek.Open(nil, sp.WrappedDEK[1:13], sp.WrappedDEK[13:], []byte("space_dek:"+sp.ID))
	if e != nil || len(dek) != 32 {
		return nil, code("encryption_unavailable")
	}
	return dek, nil
}

func (s *Service) spaceContentAEAD(sp Space) (cipher.AEAD, error) {
	dek, e := s.spaceDEK(sp)
	if e != nil {
		return nil, e
	}
	return contentAEAD(dek)
}

// backupAEAD seals backup bundles under a purpose-separated instance-key
// derivation so KEK rotation never invalidates encrypted backups.
func (s *Service) backupAEAD() (cipher.AEAD, error) {
	key, e := deriveKey(s.key, backupKeyInfo)
	if e != nil {
		return nil, code("encryption_unavailable")
	}
	return contentAEAD(key)
}
