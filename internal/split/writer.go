package split

import (
	"compress/gzip"
	"crypto/md5"
	"os"

	"github.com/fulmenhq/dimlox/internal/fileutil"
)

type shardStats struct {
	rows  int64
	bytes int64
	md5   []byte
}

type shardWriter struct {
	finalPath string
	tempPath  string
	file      *os.File
	gz        *gzip.Writer
	writer    *countingHashWriter
	rows      int64
	closed    bool
}

func (s *shardWriter) Abort() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	if s.gz != nil {
		_ = s.gz.Close()
	}
	if s.file != nil {
		_ = s.file.Close()
	}
	if s.tempPath != "" {
		_ = os.Remove(s.tempPath)
	}
	return nil
}

func newShardWriter(path string, compress bool) (*shardWriter, error) {
	tempPath := path + ".part"
	_ = os.Remove(tempPath)
	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	w := &countingHashWriter{w: f, hash: md5.New()}
	sw := &shardWriter{finalPath: path, tempPath: tempPath, file: f, writer: w}
	if compress {
		sw.gz = gzip.NewWriter(w)
	}
	return sw, nil
}

func (s *shardWriter) WriteHeader(line []byte) error {
	return s.write(line)
}

func (s *shardWriter) WriteRow(line []byte) error {
	if err := s.write(line); err != nil {
		return err
	}
	s.rows++
	return nil
}

func (s *shardWriter) Write(p []byte) (int, error) {
	if s.gz != nil {
		return s.gz.Write(p)
	}
	return s.writer.Write(p)
}

func (s *shardWriter) write(line []byte) error {
	if s.gz != nil {
		_, err := s.gz.Write(line)
		return err
	}
	_, err := s.writer.Write(line)
	return err
}

func (s *shardWriter) Close() (shardStats, error) {
	if s.closed {
		return shardStats{}, nil
	}
	s.closed = true
	if s.gz != nil {
		if err := s.gz.Close(); err != nil {
			return shardStats{}, err
		}
	}
	if err := s.file.Close(); err != nil {
		return shardStats{}, err
	}
	if err := fileutil.AtomicRename(s.tempPath, s.finalPath, true); err != nil {
		return shardStats{}, err
	}
	return shardStats{rows: s.rows, bytes: s.writer.n, md5: s.writer.hash.Sum(nil)}, nil
}
