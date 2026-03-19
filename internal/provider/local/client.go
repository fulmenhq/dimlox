package local

import (
	"context"
	"errors"
	"io"
	"iter"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

type Provider struct{}

func NewLocalProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "local"
}

func (p *Provider) Stat(_ context.Context, rawURI string) (*provider.ObjectMeta, error) {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(parsed.LocalPath)
	if err != nil {
		return nil, err
	}
	return objectMetaForPath(parsed.LocalPath, info.Name(), info), nil
}

func (p *Provider) List(_ context.Context, rawURI string, opts provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(yield func(*provider.ObjectMeta, error) bool) {
		parsed, err := uri.Parse(rawURI)
		if err != nil {
			yield(nil, err)
			return
		}

		info, err := os.Stat(parsed.LocalPath)
		if err != nil {
			yield(nil, err)
			return
		}

		if !info.IsDir() {
			yield(objectMetaForPath(parsed.LocalPath, info.Name(), info), nil)
			return
		}

		count := 0
		limitReached := func() bool {
			return opts.Limit > 0 && count >= opts.Limit
		}

		if opts.Recursive {
			err = filepath.WalkDir(parsed.LocalPath, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if path == parsed.LocalPath {
					return nil
				}
				if limitReached() {
					return io.EOF
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				relPath, err := filepath.Rel(parsed.LocalPath, path)
				if err != nil {
					return err
				}
				count++
				if !yield(objectMetaForPath(path, filepath.ToSlash(relPath), info), nil) {
					return io.EOF
				}
				return nil
			})
			if err != nil && !errors.Is(err, io.EOF) {
				yield(nil, err)
			}
			return
		}

		entries, err := os.ReadDir(parsed.LocalPath)
		if err != nil {
			yield(nil, err)
			return
		}
		for _, entry := range entries {
			if limitReached() {
				return
			}
			info, err := entry.Info()
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			count++
			path := filepath.Join(parsed.LocalPath, entry.Name())
			if !yield(objectMetaForPath(path, entry.Name(), info), nil) {
				return
			}
		}
	}
}

func (p *Provider) OpenReader(_ context.Context, rawURI string, offset, length int64) (io.ReadCloser, error) {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(parsed.LocalPath)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
	if length >= 0 {
		return struct {
			io.Reader
			io.Closer
		}{Reader: io.LimitReader(f, length), Closer: f}, nil
	}
	return f, nil
}

func (p *Provider) DownloadFile(_ context.Context, rawURI string, dst *os.File, opts provider.DownloadOptions) error {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return err
	}
	src, err := os.Open(parsed.LocalPath)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := provider.ResetFile(dst, -1); err != nil {
		return err
	}
	_, err = io.Copy(dst, provider.NewProgressReader(src, opts.Progress))
	return err
}

func (p *Provider) UploadFile(_ context.Context, src *os.File, rawURI string, opts provider.UploadOptions) error {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(parsed.LocalPath), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(parsed.LocalPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(dst, provider.NewProgressReader(src, opts.Progress))
	return err
}

func objectMetaForPath(path, name string, info os.FileInfo) *provider.ObjectMeta {
	normalized := "file://" + path
	contentType := ""
	isPrefix := info.IsDir()
	if isPrefix {
		contentType = "inode/directory"
	} else {
		contentType = mime.TypeByExtension(filepath.Ext(path))
	}
	return &provider.ObjectMeta{
		URI:          normalized,
		Name:         name,
		Size:         info.Size(),
		ContentType:  contentType,
		LastModified: info.ModTime().UTC().Round(time.Second),
		IsPrefix:     isPrefix,
		Raw: map[string]any{
			"path": path,
			"mode": info.Mode().String(),
		},
	}
}
