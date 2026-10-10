package services

import (
	"io"
)

// limitedBody caps a response body's size, returning errTooLarge once exceeded.
type limitedBody struct {
	inner       io.ReadCloser
	remaining   int64
	errTooLarge error
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		// The limit was consumed exactly; only fail if more data follows.
		var probe [1]byte
		n, err := b.inner.Read(probe[:])
		if n > 0 {
			return 0, b.errTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.inner.Read(p)
	b.remaining -= int64(n)
	return n, err
}

func (b *limitedBody) Close() error { return b.inner.Close() }
