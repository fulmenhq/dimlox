package provider

import (
	"io"
	"os"
	"sync/atomic"
)

// NewCountingReader wraps r and calls onRead with the number of bytes just read.
// If onRead is nil, r is returned unchanged.
func NewCountingReader(r io.Reader, onRead func(n int)) io.Reader {
	if onRead == nil {
		return r
	}
	return &progressReader{reader: r, onRead: onRead}
}

// NewProgressReader wraps r and reports the running byte total after each read.
// If progress is nil, r is returned unchanged.
func NewProgressReader(r io.Reader, progress func(bytesTransferred int64)) io.Reader {
	if progress == nil {
		return r
	}
	pr := &progressReader{reader: r}
	pr.onRead = func(n int) {
		progress(pr.total.Add(int64(n)))
	}
	return pr
}

// ResetFile prepares f for writing. When size >= 0, the file is truncated to that
// size first; when size < 0, it is truncated to zero length.
func ResetFile(f *os.File, size int64) error {
	if size < 0 {
		size = 0
	}
	if err := f.Truncate(size); err != nil {
		return err
	}
	_, err := f.Seek(0, io.SeekStart)
	return err
}

type progressReader struct {
	reader io.Reader
	onRead func(int)
	total  atomic.Int64
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.onRead != nil {
		r.onRead(n)
	}
	return n, err
}
