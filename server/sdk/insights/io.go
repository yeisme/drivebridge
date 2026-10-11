package insights

import (
	"io"
)

// ObservedReader measures bytes consumed by an application boundary. SDK
// retries that bypass this reader are explicitly outside its coverage.
type ObservedReader struct {
	Reader io.Reader
	Bytes  int64
}

func (r *ObservedReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	r.Bytes += int64(n)
	return n, e
}

type observedCloser struct {
	io.ReadCloser
	store  *Store
	fact   Fact
	read   int64
	closed bool
}

func (r *observedCloser) Read(p []byte) (int, error) {
	n, e := r.ReadCloser.Read(p)
	r.read += int64(n)
	return n, e
}
func (r *observedCloser) Close() error {
	if !r.closed {
		r.closed = true
		r.fact.Bytes = r.read
		_ = r.store.Record(r.fact)
	}
	return r.ReadCloser.Close()
}
func (s *Store) Observe(body io.ReadCloser, f Fact) io.ReadCloser {
	return &observedCloser{ReadCloser: body, store: s, fact: f}
}

// Preserve seekability required by S3 signing/retry middleware. Observations
// include SDK body rereads and therefore are not wire-byte billing counters.
type ObservedReadSeeker struct {
	io.ReadSeeker
	Bytes int64
}

func (r *ObservedReadSeeker) Read(p []byte) (int, error) {
	n, e := r.ReadSeeker.Read(p)
	r.Bytes += int64(n)
	return n, e
}
