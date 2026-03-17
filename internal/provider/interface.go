// Package provider defines the StorageProvider interface that all storage
// backends (AZBlob, GCS, local) must implement. This is the single
// cross-cutting contract for dimlox. See dimlox-plan.md §4.
//
// Rule: do not add methods to this interface speculatively. Add only what the
// current phase requires. Changes here require architect review.
package provider

import (
	"context"
	"io"
	"iter"
	"os"
	"time"
)

// ObjectMeta describes a single object in any storage system.
type ObjectMeta struct {
	// URI is the normalized dimlox URI for this object.
	URI string

	// Name is the object key / blob name / file base name.
	Name string

	// Size is the object size in bytes. -1 if unknown.
	Size int64

	// ETag is the provider ETag string. Empty if not available.
	ETag string

	// ContentType is the MIME type. Empty if not available.
	ContentType string

	// MD5 is the MD5 checksum of the object content. Nil if not provided.
	MD5 []byte

	// CRC32C is the CRC32C checksum (GCS). Zero if not available.
	CRC32C uint32

	// LastModified is the last-modified timestamp. Zero if not available.
	LastModified time.Time

	// IsPrefix is true for "directory" or "prefix" entries in list results.
	IsPrefix bool

	// Raw holds provider-specific raw metadata for doctor/debug output.
	Raw any
}

// ListOptions controls the behaviour of StorageProvider.List.
type ListOptions struct {
	// Recursive lists all objects under the prefix, not just immediate children.
	Recursive bool

	// Limit caps the number of results. 0 means unlimited.
	Limit int
}

// DownloadOptions controls the behaviour of StorageProvider.DownloadFile.
type DownloadOptions struct {
	// BlockSize is the chunk size in bytes for parallel downloads.
	// 0 uses the provider default.
	BlockSize int64

	// Concurrency is the number of parallel chunks. 0 uses the provider default.
	Concurrency int

	// Progress is called with the running total of bytes transferred.
	// It may be called from multiple goroutines and must be concurrency-safe.
	Progress func(bytesTransferred int64)
}

// UploadOptions controls the behaviour of StorageProvider.UploadFile.
type UploadOptions struct {
	// BlockSize is the chunk size in bytes for multipart uploads.
	BlockSize int64

	// Concurrency is the number of parallel upload workers.
	Concurrency int

	// ContentType overrides the detected MIME type. Empty means auto-detect.
	ContentType string

	// Progress is called with the running total of bytes transferred.
	Progress func(bytesTransferred int64)
}

// StorageProvider is the single interface every storage backend must satisfy.
// All dimlox commands depend on this interface; nothing else is exported from
// individual provider packages.
type StorageProvider interface {
	// Name returns a short human-readable provider identifier ("azblob", "gcs", "local").
	Name() string

	// Stat returns metadata for a single object. Returns an error wrapping
	// os.ErrNotExist if the object does not exist.
	Stat(ctx context.Context, uri string) (*ObjectMeta, error)

	// List returns an iterator over objects matching prefix. Pagination is
	// handled internally. The iterator yields (meta, err) pairs; callers
	// should stop on the first non-nil error.
	//
	// Uses iter.Seq2 from Go 1.23 range-over-func. If the build target is
	// Go 1.22, replace with a callback signature and update this comment.
	List(ctx context.Context, prefix string, opts ListOptions) iter.Seq2[*ObjectMeta, error]

	// OpenReader opens a ranged read starting at offset. length=-1 reads to EOF.
	// The caller is responsible for closing the returned ReadCloser.
	OpenReader(ctx context.Context, uri string, offset, length int64) (io.ReadCloser, error)

	// DownloadFile downloads the object at uri to dst using provider-native
	// parallelism where available (AZBlob SDK parallel blocks, GCS range reads).
	DownloadFile(ctx context.Context, uri string, dst *os.File, opts DownloadOptions) error

	// UploadFile uploads src to the object at uri using provider-native
	// multipart upload where available.
	UploadFile(ctx context.Context, src *os.File, uri string, opts UploadOptions) error
}
