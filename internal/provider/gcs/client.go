package gcs

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"iter"
	"os"
	"strings"

	storageapi "cloud.google.com/go/storage"
	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
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

func (p *Provider) OpenReader(context.Context, string, int64, int64) (io.ReadCloser, error) {
	return nil, provider.ErrNotImplemented
}

func (p *Provider) DownloadFile(context.Context, string, *os.File, provider.DownloadOptions) error {
	return provider.ErrNotImplemented
}

func (p *Provider) UploadFile(context.Context, *os.File, string, provider.UploadOptions) error {
	return provider.ErrNotImplemented
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
