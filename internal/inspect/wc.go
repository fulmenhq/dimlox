package inspect

import (
	"bufio"
	"bytes"
	"context"
	"io"

	"github.com/fulmenhq/dimlox/internal/provider"
)

type WCResult struct {
	URI             string `json:"uri"`
	CompressedBytes int64  `json:"compressed_bytes"`
	Lines           int64  `json:"lines"`
	Compressed      bool   `json:"compressed"`
}

func WC(ctx context.Context, rawURI string, opts ProviderOptions) (*WCResult, error) {
	src, _, err := providerForURI(ctx, rawURI, opts)
	if err != nil {
		return nil, err
	}
	meta, err := src.Stat(ctx, rawURI)
	if err != nil {
		return nil, err
	}
	r, compressed, err := openStream(ctx, src, rawURI, meta)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	lines, err := countLines(r)
	if err != nil {
		return nil, err
	}
	return &WCResult{
		URI:             rawURI,
		CompressedBytes: meta.Size,
		Lines:           lines,
		Compressed:      compressed,
	}, nil
}

func countLines(r io.Reader) (int64, error) {
	br := bufio.NewReaderSize(r, 4*1024*1024)
	var lines int64
	var sawData bool
	var lastByte byte
	for {
		chunk, err := br.ReadSlice('\n')
		if len(chunk) > 0 {
			sawData = true
			lines += int64(bytes.Count(chunk, []byte{'\n'}))
			lastByte = chunk[len(chunk)-1]
		}
		if err == nil {
			continue
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF {
			if sawData && lastByte != '\n' {
				lines++
			}
			return lines, nil
		}
		return 0, err
	}
}

func WCFromMeta(meta *provider.ObjectMeta, lines int64, compressed bool) *WCResult {
	return &WCResult{CompressedBytes: meta.Size, Lines: lines, Compressed: compressed}
}
