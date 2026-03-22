package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/fulmenhq/dimlox/internal/uri"
)

const (
	exitSuccess          = 0
	exitOperational      = 1
	exitBadURI           = 2
	exitChecksumMismatch = 3
	exitDiskFull         = 4
)

type exitError struct {
	code int
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

func withExitCode(code int, format string, args ...any) error {
	if format == "%v" && len(args) == 1 {
		if err, ok := args[0].(error); ok {
			return &exitError{code: code, err: err}
		}
	}
	return &exitError{code: code, err: fmt.Errorf(format, args...)}
}

func exitCodeFor(err error) int {
	if err == nil {
		return exitSuccess
	}
	if isDiskFullError(err) {
		return exitDiskFull
	}
	if isBadInputError(err) {
		return exitBadURI
	}
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return exitOperational
}

func isBadInputError(err error) bool {
	if err == nil {
		return false
	}
	var unsupported *uri.ErrUnsupportedScheme
	if errors.Is(err, uri.ErrEmptyURI) || errors.As(err, &unsupported) {
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
		os.Exit(exitSuccess)
	}
	_, _ = fmt.Fprintf(os.Stderr, "dimlox: %v\n", err)
	os.Exit(exitCodeFor(err))
}
