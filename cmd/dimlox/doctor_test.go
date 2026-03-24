package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/fulmenhq/dimlox/internal/doctor"
	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
	"github.com/fulmenhq/gofulmen/foundry"
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

func TestDoctorListGCPProfilesRejectsTargetURI(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"doctor", "--list-gcp-profiles", "gs://bucket/object"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if got := exitCodeFor(err); got != foundry.ExitInvalidArgument {
		t.Fatalf("exitCodeFor(doctor --list-gcp-profiles uri) = %d, want %d", got, foundry.ExitInvalidArgument)
	}
}

func TestDoctorAuthFailureUsesAuthenticationExitCode(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"doctor"})
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	origRun := doctorRunFunc
	t.Cleanup(func() { doctorRunFunc = origRun })
	doctorRunFunc = func(_ context.Context, _ string, _ doctor.Options) (*doctor.Result, error) {
		return &doctor.Result{Statuses: []doctor.Status{{Provider: "gcs", OK: false, Kind: "auth", Detail: "adc missing"}}}, errors.New("doctor checks failed")
	}

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if got := exitCodeFor(err); got != foundry.ExitAuthenticationFailed {
		t.Fatalf("exitCodeFor(doctor auth failure) = %d, want %d", got, foundry.ExitAuthenticationFailed)
	}
}
