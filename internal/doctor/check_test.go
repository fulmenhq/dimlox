package doctor

import (
	"testing"

	"github.com/fulmenhq/dimlox/internal/uri"
)

func TestShouldUseListProbe(t *testing.T) {
	tests := []struct {
		name   string
		target string
		parsed *uri.ParsedURI
		want   bool
	}{
		{
			name:   "gcs bucket without slash uses list probe",
			target: "gs://bucket",
			parsed: &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: ""},
			want:   true,
		},
		{
			name:   "gcs bucket with slash uses list probe",
			target: "gs://bucket/",
			parsed: &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: ""},
			want:   true,
		},
		{
			name:   "gcs object uses stat probe",
			target: "gs://bucket/object.txt",
			parsed: &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "object.txt"},
			want:   false,
		},
		{
			name:   "azblob container without slash uses list probe",
			target: "azblob://acct/container",
			parsed: &uri.ParsedURI{Provider: uri.ProviderAZBlob, AZAccount: "acct", AZContainer: "container", AZBlobPath: ""},
			want:   true,
		},
		{
			name:   "azblob blob uses stat probe",
			target: "azblob://acct/container/path/file.csv",
			parsed: &uri.ParsedURI{Provider: uri.ProviderAZBlob, AZAccount: "acct", AZContainer: "container", AZBlobPath: "path/file.csv"},
			want:   false,
		},
		{
			name:   "local path uses stat probe",
			target: "/tmp/example",
			parsed: &uri.ParsedURI{Provider: uri.ProviderLocal, LocalPath: "/tmp/example"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseListProbe(tt.target, tt.parsed)
			if got != tt.want {
				t.Fatalf("shouldUseListProbe(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}
