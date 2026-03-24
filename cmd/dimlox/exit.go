package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	providerazblob "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
	"github.com/fulmenhq/dimlox/internal/transfer"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/fulmenhq/gofulmen/foundry"
)

type exitError struct {
	code foundry.ExitCode
	err  error
}

func (e *exitError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func withExitCode(code foundry.ExitCode, format string, args ...any) error {
	if format == "%v" && len(args) == 1 {
		if err, ok := args[0].(error); ok {
			return &exitError{code: code, err: err}
		}
	}
	return &exitError{code: code, err: fmt.Errorf(format, args...)}
}

func exitCodeFor(err error) foundry.ExitCode {
	if err == nil {
		return foundry.ExitSuccess
	}
	// Priority chain for phase 1 DIM-004 alignment:
	// 1) disk full, 2) checksum mismatch, 3) auth, 4) bad input,
	// 5) explicit exitError wrapper, 6) generic failure.
	if isDiskFullError(err) {
		return foundry.ExitResourceExhausted
	}
	if errors.Is(err, transfer.ErrChecksumMismatch) {
		return foundry.ExitDataCorrupt
	}
	if isAuthError(err) {
		return foundry.ExitAuthenticationFailed
	}
	if isBadInputError(err) {
		return foundry.ExitInvalidArgument
	}
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return foundry.ExitFailure
}

func isBadInputError(err error) bool {
	if err == nil {
		return false
	}
	var unsupported *uri.ErrUnsupportedScheme
	if errors.Is(err, uri.ErrEmptyURI) || errors.Is(err, providergcs.ErrProfileNotFound) || errors.As(err, &unsupported) {
		return true
	}
	msg := err.Error()
	for _, marker := range []string{
		"invalid --format ",
		"invalid --out-fmt ",
		"invalid split mode ",
		"split requires --rows > 0 or --bytes > 0",
		"inspect requires one of ",
		"accepts ",
		"requires at least ",
		"requires at most ",
		"requires between ",
		"unknown flag: ",
		"unknown command ",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, providergcs.ErrADCMissing) {
		return true
	}
	var authFailed *providerazblob.AuthenticationFailedError
	if errors.As(err, &authFailed) {
		return true
	}
	var authRequired *providerazblob.AuthenticationRequiredError
	if errors.As(err, &authRequired) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"defaultazurecredential",
		"azureclicredential",
		"please run 'az login'",
		"please run \"az login\"",
		"application default credentials",
		"google_application_credentials",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func isDiskFullError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ENOSPC) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "no space left on device")
}

func exitWithError(err error) {
	if err == nil {
		os.Exit(foundry.ExitSuccess)
	}
	_, _ = fmt.Fprintf(os.Stderr, "dimlox: %v\n", err)
	os.Exit(exitCodeFor(err))
}
