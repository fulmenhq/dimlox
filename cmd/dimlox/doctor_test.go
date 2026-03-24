package main

import (
	"bytes"
	"testing"

	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
)

func TestDoctorListGCPProfilesOutputsLocalProfiles(t *testing.T) {
	orig := listGCPProfilesFunc
	t.Cleanup(func() { listGCPProfilesFunc = orig })

	listGCPProfilesFunc = func() (*providergcs.ProfileList, error) {
		return &providergcs.ProfileList{
			ConfigDir: "/tmp/gcloud",
			Profiles: []providergcs.Profile{
				{Name: "default", Account: "user@example.com", Project: "default-project"},
				{Name: "project-a", Account: "svc@example.com", Project: "proj-a", CredentialFileOverride: "/tmp/project-a.json"},
			},
		}, nil
	}

	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"doctor", "--list-gcp-profiles"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	got := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("gcp profiles (from /tmp/gcloud/configurations/)")) {
		t.Fatalf("stdout = %q, want heading", got)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("project-a")) || !bytes.Contains(stdout.Bytes(), []byte("credential_file_override=/tmp/project-a.json")) {
		t.Fatalf("stdout = %q, want profile details", got)
	}
}
