package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRootVersionFlag(t *testing.T) {
	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(--version) error = %v", err)
	}

	want := "dimlox " + formatVersion() + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionSubcommandAlias(t *testing.T) {
	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(version) error = %v", err)
	}

	want := "dimlox " + formatVersion() + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestLandingFlagDefaultsFromEnv(t *testing.T) {
	const landing = "/tmp/dimlox-landing-test"

	old, had := os.LookupEnv("DIMLOX_LANDING_DIR")
	if err := os.Setenv("DIMLOX_LANDING_DIR", landing); err != nil {
		t.Fatalf("Setenv DIMLOX_LANDING_DIR: %v", err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("DIMLOX_LANDING_DIR", old)
			return
		}
		_ = os.Unsetenv("DIMLOX_LANDING_DIR")
	})

	cmd := rootCmd()
	flag := cmd.PersistentFlags().Lookup("landing")
	if flag == nil {
		t.Fatal("landing flag not found")
	}
	if flag.DefValue != landing {
		t.Fatalf("landing default = %q, want %q", flag.DefValue, landing)
	}
}

func TestGCPProjectDefaultsFromEnv(t *testing.T) {
	const project = "dimlox-test-project"

	oldCloud, hadCloud := os.LookupEnv("GCLOUD_PROJECT")
	oldGoogle, hadGoogle := os.LookupEnv("GOOGLE_CLOUD_PROJECT")
	_ = os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	if err := os.Setenv("GCLOUD_PROJECT", project); err != nil {
		t.Fatalf("Setenv GCLOUD_PROJECT: %v", err)
	}
	t.Cleanup(func() {
		if hadCloud {
			_ = os.Setenv("GCLOUD_PROJECT", oldCloud)
		} else {
			_ = os.Unsetenv("GCLOUD_PROJECT")
		}
		if hadGoogle {
			_ = os.Setenv("GOOGLE_CLOUD_PROJECT", oldGoogle)
		} else {
			_ = os.Unsetenv("GOOGLE_CLOUD_PROJECT")
		}
	})

	cmd := rootCmd()
	flag := cmd.PersistentFlags().Lookup("gcp-project")
	if flag == nil {
		t.Fatal("gcp-project flag not found")
	}
	if flag.DefValue != project {
		t.Fatalf("gcp-project default = %q, want %q", flag.DefValue, project)
	}
}

func TestDoctorBadURIExitCode(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"doctor", "s3://bucket/key"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want exit error")
	}
	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("Execute() error = %T, want *exitError", err)
	}
	if ee.code != exitBadURI {
		t.Fatalf("exit code = %d, want %d", ee.code, exitBadURI)
	}
}

func TestGetMissingArgsExitCode(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"get"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if got := exitCodeFor(err); got != exitBadURI {
		t.Fatalf("exitCodeFor(get missing args) = %d, want %d (err=%v)", got, exitBadURI, err)
	}
}

func TestSplitInvalidLimitsExitCode(t *testing.T) {
	src := filepath.Join(t.TempDir(), "sample.psv")
	if err := os.WriteFile(src, []byte("a|b\n1|2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cmd := rootCmd()
	cmd.SetArgs([]string{"split", "--rows", "0", "--bytes", "0", src})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if got := exitCodeFor(err); got != exitBadURI {
		t.Fatalf("exitCodeFor(split invalid limits) = %d, want %d (err=%v)", got, exitBadURI, err)
	}
}

func TestExitCodeForDiskFull(t *testing.T) {
	err := withExitCode(exitOperational, "%v", &os.PathError{Op: "write", Path: "/tmp/out", Err: syscall.ENOSPC})
	if got := exitCodeFor(err); got != exitDiskFull {
		t.Fatalf("exitCodeFor(disk full) = %d, want %d", got, exitDiskFull)
	}
}
