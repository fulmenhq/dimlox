package split

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/dimlox/internal/inspect"
	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/providers"
	"github.com/fulmenhq/dimlox/internal/uri"
)

type ProviderOptions = providers.Options

type Mode string

const (
	ModeAuto   Mode = "auto"
	ModeStream Mode = "stream"
	ModeRange  Mode = "range"
	ModeBinary Mode = "binary"
)

type Options struct {
	ProviderOptions
	Mode        Mode
	Rows        int64
	Bytes       int64
	OutDir      string
	OutFmt      string
	Header      bool
	Delimiter   string
	Encoding    string
	Manifest    bool
	DryRun      bool
	SampleBytes int64
}

type Result struct {
	SourceURI    string                `json:"source_uri"`
	Mode         Mode                  `json:"mode"`
	OutDir       string                `json:"out_dir"`
	ManifestPath string                `json:"manifest_path,omitempty"`
	DryRun       bool                  `json:"dry_run"`
	Delimiter    string                `json:"delimiter,omitempty"`
	Encoding     string                `json:"encoding,omitempty"`
	HeaderCopied bool                  `json:"header_copied"`
	Shards       []ManifestEntry       `json:"shards"`
	Detected     *inspect.DetectResult `json:"detected,omitempty"`
}

var providerResolver = providers.ForURI

func providerForURI(ctx context.Context, rawURI string, opts ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
	return providerResolver(ctx, rawURI, providers.Options(opts))
}

func openStream(ctx context.Context, src provider.StorageProvider, rawURI string, meta *provider.ObjectMeta) (io.ReadCloser, bool, error) {
	r, err := src.OpenReader(ctx, rawURI, 0, -1)
	if err != nil {
		return nil, false, err
	}
	if !isCompressed(rawURI, meta) {
		return r, false, nil
	}
	gz, err := gzip.NewReader(r)
	if err != nil {
		_ = r.Close()
		return nil, true, err
	}
	return &combinedReadCloser{Reader: gz, closers: []io.Closer{gz, r}}, true, nil
}

func openRawStream(ctx context.Context, src provider.StorageProvider, rawURI string) (io.ReadCloser, error) {
	return src.OpenReader(ctx, rawURI, 0, -1)
}

func isCompressed(rawURI string, meta *provider.ObjectMeta) bool {
	if meta != nil && strings.EqualFold(meta.ContentType, "application/gzip") {
		return true
	}
	parsed, err := uri.Parse(rawURI)
	if err == nil && parsed.Provider == uri.ProviderLocal {
		return strings.EqualFold(filepath.Ext(parsed.LocalPath), ".gz")
	}
	return strings.EqualFold(filepath.Ext(rawURI), ".gz")
}

type combinedReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *combinedReadCloser) Close() error {
	var firstErr error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func Split(ctx context.Context, rawURI string, opts Options) (*Result, error) {
	if opts.Bytes <= 0 && opts.Rows <= 0 {
		return nil, fmt.Errorf("split requires --rows > 0 or --bytes > 0")
	}
	if opts.SampleBytes <= 0 {
		opts.SampleBytes = 65536
	}
	if opts.Mode == "" {
		opts.Mode = ModeAuto
	}
	if opts.OutFmt == "" {
		opts.OutFmt = "match"
	}

	src, parsed, err := providerForURI(ctx, rawURI, opts.ProviderOptions)
	if err != nil {
		return nil, err
	}
	meta, err := src.Stat(ctx, rawURI)
	if err != nil {
		return nil, err
	}
	outDir, err := resolveOutDir(opts.OutDir)
	if err != nil {
		return nil, err
	}

	mode, err := resolveMode(rawURI, parsed, meta, opts.Mode)
	if err != nil {
		return nil, err
	}
	resolved := opts
	resolved.Mode = mode

	switch mode {
	case ModeStream:
		return Stream(ctx, rawURI, src, parsed, meta, outDir, resolved)
	case ModeBinary:
		return Binary(ctx, rawURI, src, parsed, meta, outDir, resolved)
	case ModeRange:
		return Range(ctx, rawURI, src, parsed, meta, outDir, resolved)
	default:
		return nil, fmt.Errorf("unsupported split mode %q", mode)
	}
}

func resolveMode(rawURI string, parsed *uri.ParsedURI, meta *provider.ObjectMeta, requested Mode) (Mode, error) {
	switch requested {
	case ModeStream, ModeRange, ModeBinary:
		return requested, nil
	case ModeAuto:
		if isCompressed(rawURI, meta) {
			return ModeStream, nil
		}
		if parsed != nil && parsed.Provider != uri.ProviderLocal && isTextLike(rawURI, meta) {
			return ModeRange, nil
		}
		if isTextLike(rawURI, meta) {
			return ModeStream, nil
		}
		return ModeBinary, nil
	default:
		return "", fmt.Errorf("invalid split mode %q (want auto|stream|range|binary)", requested)
	}
}

func isTextLike(rawURI string, meta *provider.ObjectMeta) bool {
	if meta != nil && strings.HasPrefix(strings.ToLower(meta.ContentType), "text/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(strings.TrimSuffix(rawURI, ".gz"))) {
	case ".csv", ".psv", ".tsv", ".json", ".txt":
		return true
	default:
		return false
	}
}

func resolveOutDir(override string) (string, error) {
	path := override
	if path == "" {
		path = os.Getenv("DIMLOX_LANDING_DIR")
	}
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = cwd
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("output directory does not exist: %s", path)
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("output path is not a directory: %s", path)
	}
	return filepath.Clean(path), nil
}

func sourceBaseName(rawURI string, parsed *uri.ParsedURI, meta *provider.ObjectMeta) string {
	if meta != nil && meta.Name != "" {
		return filepath.Base(meta.Name)
	}
	if parsed == nil {
		return filepath.Base(rawURI)
	}
	switch parsed.Provider {
	case uri.ProviderAZBlob:
		return filepath.Base(parsed.AZBlobPath)
	case uri.ProviderGCS:
		return filepath.Base(parsed.GCSObject)
	case uri.ProviderLocal:
		return filepath.Base(parsed.LocalPath)
	default:
		return filepath.Base(rawURI)
	}
}

func textStemAndExt(name string) (string, string) {
	trimmed := name
	if strings.HasSuffix(strings.ToLower(trimmed), ".gz") {
		trimmed = strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
	}
	ext := filepath.Ext(trimmed)
	stem := strings.TrimSuffix(trimmed, ext)
	if stem == "" {
		stem = trimmed
	}
	return stem, ext
}

func textShardPath(outDir, sourceName string, index int, compress bool) string {
	stem, ext := textStemAndExt(sourceName)
	name := fmt.Sprintf("%s_shard_%04d%s", stem, index, ext)
	if compress {
		name += ".gz"
	}
	return filepath.Join(outDir, name)
}

func binaryShardPath(outDir, sourceName string, index int) string {
	stem, _ := textStemAndExt(sourceName)
	return filepath.Join(outDir, fmt.Sprintf("%s_part_%04d.bin", stem, index))
}

func manifestPath(outDir, sourceName string) string {
	stem, _ := textStemAndExt(sourceName)
	return filepath.Join(outDir, fmt.Sprintf("%s_manifest.jsonl", stem))
}

func resolveOutputCompression(sourceCompressed bool, outFmt string) (bool, error) {
	switch strings.ToLower(outFmt) {
	case "", "match":
		return sourceCompressed, nil
	case "text":
		return false, nil
	case "gz":
		return true, nil
	default:
		return false, fmt.Errorf("invalid --out-fmt %q (want match|text|gz)", outFmt)
	}
}

func resolveDetection(ctx context.Context, rawURI string, opts Options) (string, string, *inspect.DetectResult, error) {
	delimiter := opts.Delimiter
	encoding := opts.Encoding
	if delimiter != "" && encoding != "" {
		return delimiter, encoding, nil, nil
	}
	res, err := inspect.Detect(ctx, rawURI, opts.SampleBytes, inspect.ProviderOptions(opts.ProviderOptions))
	if err != nil {
		return "", "", nil, err
	}
	if delimiter == "" {
		delimiter = res.Delimiter
	}
	if encoding == "" {
		encoding = res.Encoding
	}
	return delimiter, encoding, res, nil
}
