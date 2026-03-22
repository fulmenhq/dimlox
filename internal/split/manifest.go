package split

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"time"

	"github.com/fulmenhq/dimlox/internal/fileutil"
	"github.com/fulmenhq/dimlox/internal/provider"
)

type ManifestEntry struct {
	SourceURI    string `json:"source_uri"`
	SourceETag   string `json:"source_etag,omitempty"`
	SourceSize   int64  `json:"source_size"`
	ShardIndex   int    `json:"shard_index"`
	ShardFile    string `json:"shard_file"`
	ShardRows    int64  `json:"shard_rows"`
	ShardBytes   int64  `json:"shard_bytes"`
	ShardMD5     string `json:"shard_md5,omitempty"`
	SplitMode    Mode   `json:"split_mode"`
	Delimiter    string `json:"delimiter,omitempty"`
	Encoding     string `json:"encoding,omitempty"`
	HeaderCopied bool   `json:"header_copied"`
	CompletedAt  string `json:"completed_at"`
}

type manifestWriter struct {
	path   string
	temp   string
	file   *os.File
	enc    *json.Encoder
	dryRun bool
	closed bool
}

func (m *manifestWriter) Abort() error {
	if m == nil || m.dryRun || m.closed {
		return nil
	}
	m.closed = true
	if m.file != nil {
		_ = m.file.Close()
	}
	if m.temp != "" {
		_ = os.Remove(m.temp)
	}
	return nil
}

func newManifestWriter(path string, enabled, dryRun bool) (*manifestWriter, error) {
	if !enabled {
		return nil, nil
	}
	if dryRun {
		return &manifestWriter{path: path, dryRun: true}, nil
	}
	temp := path + ".part"
	_ = os.Remove(temp)
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	return &manifestWriter{path: path, temp: temp, file: f, enc: json.NewEncoder(f)}, nil
}

func (m *manifestWriter) Path() string {
	if m == nil {
		return ""
	}
	return m.path
}

func (m *manifestWriter) Write(entry ManifestEntry) error {
	if m == nil || m.dryRun {
		return nil
	}
	if m.closed {
		return fmt.Errorf("manifest writer already closed")
	}
	return m.enc.Encode(entry)
}

func (m *manifestWriter) Close() error {
	if m == nil || m.dryRun || m.closed {
		return nil
	}
	m.closed = true
	if err := m.file.Close(); err != nil {
		return err
	}
	return fileutil.AtomicRename(m.temp, m.path, true)
}

func buildManifestEntry(rawURI string, meta *provider.ObjectMeta, outDir, shardPath string, index int, stats shardStats, mode Mode, delimiter, encoding string, header bool) ManifestEntry {
	relPath, err := filepath.Rel(outDir, shardPath)
	if err != nil {
		relPath = shardPath
	}
	entry := ManifestEntry{
		SourceURI:    rawURI,
		ShardIndex:   index,
		ShardFile:    filepath.ToSlash(filepath.Clean(relPath)),
		ShardRows:    stats.rows,
		ShardBytes:   stats.bytes,
		SplitMode:    mode,
		Delimiter:    delimiter,
		Encoding:     encoding,
		HeaderCopied: header,
		CompletedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if meta != nil {
		entry.SourceSize = meta.Size
		entry.SourceETag = meta.ETag
	}
	if len(stats.md5) > 0 {
		entry.ShardMD5 = hex.EncodeToString(stats.md5)
	}
	return entry
}

type countingHashWriter struct {
	w    *os.File
	hash hash.Hash
	n    int64
}

func (w *countingHashWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if n > 0 {
		w.n += int64(n)
		_, _ = w.hash.Write(p[:n])
	}
	return n, err
}
