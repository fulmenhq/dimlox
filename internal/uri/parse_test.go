package uri

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	relPath, err := filepath.Abs("./relative/file.csv")
	if err != nil {
		t.Fatalf("resolve relative path: %v", err)
	}

	tests := []struct {
		name           string
		input          string
		wantProvider   Provider
		wantNormalized string
		wantAZAccount  string
		wantContainer  string
		wantBlobPath   string
		wantBucket     string
		wantObject     string
		wantLocalPath  string
		wantErr        error
		wantSchemeErr  bool
		errContains    string
	}{
		{
			name:           "azure https object",
			input:          "https://prodfilemagesa.blob.core.windows.net/filemagefs01/a/b.gz",
			wantProvider:   ProviderAZBlob,
			wantNormalized: "azblob://prodfilemagesa/filemagefs01/a/b.gz",
			wantAZAccount:  "prodfilemagesa",
			wantContainer:  "filemagefs01",
			wantBlobPath:   "a/b.gz",
		},
		{
			name:           "azure native object",
			input:          "azblob://myaccount/mycontainer/path/file.csv",
			wantProvider:   ProviderAZBlob,
			wantNormalized: "azblob://myaccount/mycontainer/path/file.csv",
			wantAZAccount:  "myaccount",
			wantContainer:  "mycontainer",
			wantBlobPath:   "path/file.csv",
		},
		{
			name:           "gcs https object",
			input:          "https://storage.googleapis.com/mybucket/path/file.csv",
			wantProvider:   ProviderGCS,
			wantNormalized: "gcs://mybucket/path/file.csv",
			wantBucket:     "mybucket",
			wantObject:     "path/file.csv",
		},
		{
			name:           "gs object",
			input:          "gs://mybucket/path/file.csv",
			wantProvider:   ProviderGCS,
			wantNormalized: "gcs://mybucket/path/file.csv",
			wantBucket:     "mybucket",
			wantObject:     "path/file.csv",
		},
		{
			name:           "absolute local path",
			input:          "/absolute/path/file.csv",
			wantProvider:   ProviderLocal,
			wantNormalized: "file:///absolute/path/file.csv",
			wantLocalPath:  "/absolute/path/file.csv",
		},
		{
			name:           "relative local path",
			input:          "./relative/file.csv",
			wantProvider:   ProviderLocal,
			wantNormalized: "file://" + relPath,
			wantLocalPath:  relPath,
		},
		{
			name:           "file uri",
			input:          "file:///path/to/file",
			wantProvider:   ProviderLocal,
			wantNormalized: "file:///path/to/file",
			wantLocalPath:  "/path/to/file",
		},
		{
			name:           "azure blob path trailing slash stripped",
			input:          "azblob://acct/container/path/to/blob/",
			wantProvider:   ProviderAZBlob,
			wantNormalized: "azblob://acct/container/path/to/blob",
			wantAZAccount:  "acct",
			wantContainer:  "container",
			wantBlobPath:   "path/to/blob",
		},
		{
			name:           "azure container trailing slash preserved",
			input:          "azblob://acct/container/",
			wantProvider:   ProviderAZBlob,
			wantNormalized: "azblob://acct/container/",
			wantAZAccount:  "acct",
			wantContainer:  "container",
			wantBlobPath:   "",
		},
		{
			name:           "gcs object trailing slash stripped",
			input:          "https://storage.googleapis.com/mybucket/path/to/object/",
			wantProvider:   ProviderGCS,
			wantNormalized: "gcs://mybucket/path/to/object",
			wantBucket:     "mybucket",
			wantObject:     "path/to/object",
		},
		{
			name:           "gcs bucket trailing slash preserved",
			input:          "https://storage.googleapis.com/mybucket/",
			wantProvider:   ProviderGCS,
			wantNormalized: "gcs://mybucket/",
			wantBucket:     "mybucket",
			wantObject:     "",
		},
		{
			name:           "percent encoding preserved",
			input:          "gs://mybucket/path%20with%20spaces/file%23name.csv",
			wantProvider:   ProviderGCS,
			wantNormalized: "gcs://mybucket/path%20with%20spaces/file%23name.csv",
			wantBucket:     "mybucket",
			wantObject:     "path%20with%20spaces/file%23name.csv",
		},
		{
			name:          "unsupported s3 scheme",
			input:         "s3://bucket/key",
			wantSchemeErr: true,
			errContains:   "unsupported URI scheme",
		},
		{
			name:          "unsupported https host",
			input:         "https://example.com/file",
			wantSchemeErr: true,
			errContains:   "unsupported URI scheme",
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     ErrEmptyURI,
			errContains: ErrEmptyURI.Error(),
		},
		{
			name:        "azure missing container",
			input:       "azblob://acct",
			errContains: "missing container",
		},
		{
			name:        "gcs missing bucket",
			input:       "gs:///path/file.csv",
			errContains: "missing bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr != nil || tt.wantSchemeErr || tt.errContains != "" {
				if err == nil {
					t.Fatalf("Parse(%q) error = nil, want error", tt.input)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("Parse(%q) error = %v, want %v", tt.input, err, tt.wantErr)
				}
				if tt.wantSchemeErr {
					var unsupported *ErrUnsupportedScheme
					if !errors.As(err, &unsupported) {
						t.Fatalf("Parse(%q) error = %v, want ErrUnsupportedScheme", tt.input, err)
					}
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("Parse(%q) error = %q, want substring %q", tt.input, err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.input, err)
			}
			if got.Provider != tt.wantProvider {
				t.Fatalf("Parse(%q) provider = %v, want %v", tt.input, got.Provider, tt.wantProvider)
			}
			if got.Normalized != tt.wantNormalized {
				t.Fatalf("Parse(%q) normalized = %q, want %q", tt.input, got.Normalized, tt.wantNormalized)
			}
			if got.AZAccount != tt.wantAZAccount {
				t.Fatalf("Parse(%q) AZAccount = %q, want %q", tt.input, got.AZAccount, tt.wantAZAccount)
			}
			if got.AZContainer != tt.wantContainer {
				t.Fatalf("Parse(%q) AZContainer = %q, want %q", tt.input, got.AZContainer, tt.wantContainer)
			}
			if got.AZBlobPath != tt.wantBlobPath {
				t.Fatalf("Parse(%q) AZBlobPath = %q, want %q", tt.input, got.AZBlobPath, tt.wantBlobPath)
			}
			if got.GCSBucket != tt.wantBucket {
				t.Fatalf("Parse(%q) GCSBucket = %q, want %q", tt.input, got.GCSBucket, tt.wantBucket)
			}
			if got.GCSObject != tt.wantObject {
				t.Fatalf("Parse(%q) GCSObject = %q, want %q", tt.input, got.GCSObject, tt.wantObject)
			}
			if got.LocalPath != tt.wantLocalPath {
				t.Fatalf("Parse(%q) LocalPath = %q, want %q", tt.input, got.LocalPath, tt.wantLocalPath)
			}
		})
	}
}
