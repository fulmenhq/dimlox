package gcs

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"iter"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	storageapi "cloud.google.com/go/storage"
	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
)

type Provider struct {
	client     *storageapi.Client
	gcpProject string
}

func NewGCSProvider(ctx context.Context, gcpProject string) (*Provider, error) {
	client, err := storageapi.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &Provider{client: client, gcpProject: gcpProject}, nil
}

func (p *Provider) Name() string {
	return "gcs"
}

func (p *Provider) Stat(ctx context.Context, rawURI string) (*provider.ObjectMeta, error) {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return nil, err
	}
	attrs, err := p.bucket(parsed.GCSBucket).Object(parsed.GCSObject).Attrs(ctx)
	if err != nil {
		return nil, err
	}
	return objectMetaFromAttrs(parsed.GCSBucket, attrs), nil
}

func (p *Provider) List(ctx context.Context, rawURI string, opts provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(yield func(*provider.ObjectMeta, error) bool) {
		parsed, err := uri.Parse(rawURI)
		if err != nil {
			yield(nil, err)
			return
		}
		query := &storageapi.Query{Prefix: listPrefix(rawURI, parsed.GCSObject)}
		if !opts.Recursive {
			query.Delimiter = "/"
		}
		it := p.bucket(parsed.GCSBucket).Objects(ctx, query)
		count := 0
		for {
			attrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				yield(nil, err)
				return
			}
			count++
			if !yield(objectMetaFromAttrs(parsed.GCSBucket, attrs), nil) {
				return
			}
			if opts.Limit > 0 && count >= opts.Limit {
				return
			}
		}
	}
}

func (p *Provider) OpenReader(ctx context.Context, rawURI string, offset, length int64) (io.ReadCloser, error) {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return nil, err
	}
	reader, err := p.bucket(parsed.GCSBucket).Object(parsed.GCSObject).ReadCompressed(true).NewRangeReader(ctx, offset, length)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func (p *Provider) DownloadFile(ctx context.Context, rawURI string, dst *os.File, opts provider.DownloadOptions) error {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return err
	}
	attrs, err := p.bucket(parsed.GCSBucket).Object(parsed.GCSObject).Attrs(ctx)
	if err != nil {
		return err
	}
	if err := resetFile(dst, attrs.Size); err != nil {
		return err
	}
	blockSize := opts.BlockSize
	if blockSize <= 0 {
		blockSize = 32 * 1024 * 1024
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 8
	}
	var total atomic.Int64
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	for offset := int64(0); offset < attrs.Size; offset += blockSize {
		offset := offset
		length := minInt64(blockSize, attrs.Size-offset)
		g.Go(func() error {
			reader, err := p.bucket(parsed.GCSBucket).Object(parsed.GCSObject).ReadCompressed(true).NewRangeReader(gctx, offset, length)
			if err != nil {
				return err
			}
			defer reader.Close()
			writer := io.NewOffsetWriter(dst, offset)
			if opts.Progress == nil {
				_, err = io.Copy(writer, reader)
				return err
			}
			_, err = io.Copy(writer, &progressReader{reader: reader, onRead: func(n int) {
				opts.Progress(total.Add(int64(n)))
			}})
			return err
		})
	}
	return g.Wait()
}

func (p *Provider) UploadFile(ctx context.Context, src *os.File, rawURI string, opts provider.UploadOptions) error {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return err
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	writer := p.bucket(parsed.GCSBucket).Object(parsed.GCSObject).NewWriter(ctx)
	if opts.BlockSize > 0 {
		writer.ChunkSize = roundChunkSize(opts.BlockSize)
	}
	if opts.Progress != nil {
		writer.ProgressFunc = opts.Progress
	}
	contentType := opts.ContentType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(parsed.GCSObject))
	}
	if contentType != "" {
		writer.ContentType = contentType
	}
	if _, err := io.Copy(writer, src); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

func (p *Provider) bucket(name string) *storageapi.BucketHandle {
	b := p.client.Bucket(name)
	if p.gcpProject != "" {
		b = b.UserProject(p.gcpProject)
	}
	return b
}

func objectMetaFromAttrs(bucket string, attrs *storageapi.ObjectAttrs) *provider.ObjectMeta {
	objectURI := fmt.Sprintf("gcs://%s", bucket)
	if attrs.Prefix != "" {
		trimmed := strings.TrimSuffix(attrs.Prefix, "/")
		return &provider.ObjectMeta{
			URI:      objectURI + "/" + trimmed,
			Name:     trimmed,
			IsPrefix: true,
			Raw:      map[string]any{"prefix": attrs.Prefix},
		}
	}
	if attrs.Name != "" {
		objectURI += "/" + attrs.Name
	}
	md5 := make([]byte, len(attrs.MD5))
	copy(md5, attrs.MD5)
	return &provider.ObjectMeta{
		URI:          objectURI,
		Name:         attrs.Name,
		Size:         attrs.Size,
		ETag:         attrs.Etag,
		ContentType:  attrs.ContentType,
		MD5:          md5,
		CRC32C:       attrs.CRC32C,
		LastModified: attrs.Updated.UTC(),
		Raw: map[string]any{
			"bucket":        attrs.Bucket,
			"storage_class": attrs.StorageClass,
			"md5_base64":    base64.StdEncoding.EncodeToString(attrs.MD5),
		},
	}
}

func listPrefix(rawURI, parsedPath string) string {
	if parsedPath == "" {
		return ""
	}
	if strings.HasSuffix(rawURI, "/") {
		return parsedPath + "/"
	}
	return parsedPath
}

func roundChunkSize(size int64) int {
	const quantum = 256 * 1024
	if size <= 0 {
		return 0
	}
	rounded := ((size + quantum - 1) / quantum) * quantum
	if rounded < quantum {
		rounded = quantum
	}
	if rounded > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(rounded)
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

type progressReader struct {
	reader io.Reader
	onRead func(int)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.onRead != nil {
		r.onRead(n)
	}
	return n, err
}

func resetFile(f *os.File, size int64) error {
	if err := f.Truncate(size); err != nil {
		return err
	}
	_, err := f.Seek(0, io.SeekStart)
	return err
}
