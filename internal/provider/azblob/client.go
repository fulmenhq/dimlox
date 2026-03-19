package azblob

import (
	"context"
	"fmt"
	"io"
	"iter"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	azsdk "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

type Provider struct {
	profile string
	cred    azcore.TokenCredential
}

func NewAZBlobProvider(_ context.Context, profile string) (*Provider, error) {
	cred, err := newCredential(profile)
	if err != nil {
		return nil, err
	}
	return &Provider{profile: profile, cred: cred}, nil
}

func (p *Provider) Name() string {
	return "azblob"
}

func (p *Provider) Stat(ctx context.Context, rawURI string) (*provider.ObjectMeta, error) {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return nil, err
	}
	client, err := p.clientForAccount(parsed.AZAccount)
	if err != nil {
		return nil, err
	}
	resp, err := client.ServiceClient().NewContainerClient(parsed.AZContainer).NewBlobClient(parsed.AZBlobPath).GetProperties(ctx, nil)
	if err != nil {
		return nil, err
	}
	return objectMetaFromProperties(parsed, resp), nil
}

func (p *Provider) List(ctx context.Context, rawURI string, opts provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(yield func(*provider.ObjectMeta, error) bool) {
		parsed, err := uri.Parse(rawURI)
		if err != nil {
			yield(nil, err)
			return
		}
		client, err := p.clientForAccount(parsed.AZAccount)
		if err != nil {
			yield(nil, err)
			return
		}
		containerClient := client.ServiceClient().NewContainerClient(parsed.AZContainer)
		prefix := listPrefix(rawURI, parsed.AZBlobPath)
		count := 0
		limit := maxResults(opts.Limit)

		if opts.Recursive {
			pager := containerClient.NewListBlobsFlatPager(&container.ListBlobsFlatOptions{Prefix: prefix, MaxResults: limit})
			for pager.More() {
				resp, err := pager.NextPage(ctx)
				if err != nil {
					yield(nil, err)
					return
				}
				for _, item := range resp.Segment.BlobItems {
					count++
					if !yield(objectMetaFromListItem(parsed.AZAccount, parsed.AZContainer, item), nil) {
						return
					}
					if opts.Limit > 0 && count >= opts.Limit {
						return
					}
				}
			}
			return
		}

		pager := containerClient.NewListBlobsHierarchyPager("/", &container.ListBlobsHierarchyOptions{Prefix: prefix, MaxResults: limit})
		for pager.More() {
			resp, err := pager.NextPage(ctx)
			if err != nil {
				yield(nil, err)
				return
			}
			for _, item := range resp.Segment.BlobItems {
				count++
				if !yield(objectMetaFromListItem(parsed.AZAccount, parsed.AZContainer, item), nil) {
					return
				}
				if opts.Limit > 0 && count >= opts.Limit {
					return
				}
			}
			for _, prefixItem := range resp.Segment.BlobPrefixes {
				count++
				if !yield(prefixMeta(parsed.AZAccount, parsed.AZContainer, *prefixItem.Name), nil) {
					return
				}
				if opts.Limit > 0 && count >= opts.Limit {
					return
				}
			}
		}
	}
}

func (p *Provider) OpenReader(ctx context.Context, rawURI string, offset, length int64) (io.ReadCloser, error) {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return nil, err
	}
	client, err := p.clientForAccount(parsed.AZAccount)
	if err != nil {
		return nil, err
	}
	stream, err := client.ServiceClient().NewContainerClient(parsed.AZContainer).NewBlobClient(parsed.AZBlobPath).DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: offset, Count: maxRangeCount(length)},
	})
	if err != nil {
		return nil, err
	}
	return stream.NewRetryReader(ctx, nil), nil
}

func (p *Provider) DownloadFile(ctx context.Context, rawURI string, dst *os.File, opts provider.DownloadOptions) error {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return err
	}
	client, err := p.clientForAccount(parsed.AZAccount)
	if err != nil {
		return err
	}
	if err := provider.ResetFile(dst, -1); err != nil {
		return err
	}
	downloadOpts := &azsdk.DownloadFileOptions{}
	if opts.BlockSize > 0 {
		downloadOpts.BlockSize = opts.BlockSize
	}
	if opts.Concurrency > 0 {
		downloadOpts.Concurrency = uint16(opts.Concurrency)
	}
	if opts.Progress != nil {
		downloadOpts.Progress = opts.Progress
	}
	_, err = client.DownloadFile(ctx, parsed.AZContainer, parsed.AZBlobPath, dst, downloadOpts)
	return err
}

func (p *Provider) UploadFile(ctx context.Context, src *os.File, rawURI string, opts provider.UploadOptions) error {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return err
	}
	client, err := p.clientForAccount(parsed.AZAccount)
	if err != nil {
		return err
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	uploadOpts := &azsdk.UploadFileOptions{}
	if opts.BlockSize > 0 {
		uploadOpts.BlockSize = opts.BlockSize
	}
	if opts.Concurrency > 0 {
		uploadOpts.Concurrency = uint16(opts.Concurrency)
	}
	if opts.Progress != nil {
		uploadOpts.Progress = opts.Progress
	}
	contentType := opts.ContentType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(parsed.AZBlobPath))
	}
	if contentType != "" {
		uploadOpts.HTTPHeaders = &blob.HTTPHeaders{BlobContentType: &contentType}
	}
	_, err = client.UploadFile(ctx, parsed.AZContainer, parsed.AZBlobPath, src, uploadOpts)
	return err
}

func (p *Provider) clientForAccount(account string) (*azsdk.Client, error) {
	client, err := azsdk.NewClient(fmt.Sprintf("https://%s.blob.core.windows.net", account), p.cred, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func objectMetaFromProperties(parsed *uri.ParsedURI, resp blob.GetPropertiesResponse) *provider.ObjectMeta {
	md5 := make([]byte, len(resp.ContentMD5))
	copy(md5, resp.ContentMD5)
	meta := &provider.ObjectMeta{
		URI:      parsed.Normalized,
		Name:     parsed.AZBlobPath,
		MD5:      md5,
		IsPrefix: false,
		Raw: map[string]any{
			"account":   parsed.AZAccount,
			"container": parsed.AZContainer,
		},
	}
	if resp.ContentLength != nil {
		meta.Size = *resp.ContentLength
	}
	if resp.ETag != nil {
		meta.ETag = string(*resp.ETag)
	}
	if resp.ContentType != nil {
		meta.ContentType = *resp.ContentType
	}
	if resp.LastModified != nil {
		meta.LastModified = resp.LastModified.UTC()
	}
	return meta
}

func objectMetaFromListItem(account, containerName string, item *container.BlobItem) *provider.ObjectMeta {
	name := ""
	if item.Name != nil {
		name = *item.Name
	}
	meta := &provider.ObjectMeta{
		URI:      fmt.Sprintf("azblob://%s/%s/%s", account, containerName, name),
		Name:     name,
		IsPrefix: false,
		Raw: map[string]any{
			"account":   account,
			"container": containerName,
		},
	}
	if item.Properties != nil {
		if item.Properties.ContentLength != nil {
			meta.Size = *item.Properties.ContentLength
		}
		if item.Properties.ETag != nil {
			meta.ETag = string(*item.Properties.ETag)
		}
		if item.Properties.ContentType != nil {
			meta.ContentType = *item.Properties.ContentType
		}
		if item.Properties.LastModified != nil {
			meta.LastModified = item.Properties.LastModified.UTC()
		}
		if len(item.Properties.ContentMD5) > 0 {
			md5 := make([]byte, len(item.Properties.ContentMD5))
			copy(md5, item.Properties.ContentMD5)
			meta.MD5 = md5
		}
	}
	return meta
}

func prefixMeta(account, containerName, name string) *provider.ObjectMeta {
	trimmed := strings.TrimSuffix(name, "/")
	return &provider.ObjectMeta{
		URI:      fmt.Sprintf("azblob://%s/%s/%s", account, containerName, trimmed),
		Name:     trimmed,
		IsPrefix: true,
		Raw:      map[string]any{"prefix": name},
	}
}

func listPrefix(rawURI, parsedPath string) *string {
	if parsedPath == "" {
		return nil
	}
	prefix := parsedPath
	if strings.HasSuffix(rawURI, "/") {
		prefix += "/"
	}
	return to.Ptr(prefix)
}

func maxResults(limit int) *int32 {
	if limit <= 0 {
		return nil
	}
	v := int32(limit)
	return &v
}

func maxRangeCount(length int64) int64 {
	if length <= 0 {
		return 0
	}
	return length
}
