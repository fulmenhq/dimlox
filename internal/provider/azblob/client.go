package azblob

import (
	"context"
	"fmt"
	"io"
	"iter"
	"os"
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

func (p *Provider) OpenReader(context.Context, string, int64, int64) (io.ReadCloser, error) {
	return nil, provider.ErrNotImplemented
}

func (p *Provider) DownloadFile(context.Context, string, *os.File, provider.DownloadOptions) error {
	return provider.ErrNotImplemented
}

func (p *Provider) UploadFile(context.Context, *os.File, string, provider.UploadOptions) error {
	return provider.ErrNotImplemented
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
