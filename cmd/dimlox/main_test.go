package main

import (
	"bytes"
	"os"
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
