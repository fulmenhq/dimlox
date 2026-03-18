package main

import (
	"errors"
	"fmt"
	"os"
)

const (
	exitSuccess     = 0
	exitOperational = 1
	exitBadURI      = 2
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
	return &exitError{code: code, err: fmt.Errorf(format, args...)}
}

func exitCodeFor(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return exitOperational
}

func exitWithError(err error) {
	if err == nil {
		os.Exit(exitSuccess)
	}
	_, _ = fmt.Fprintf(os.Stderr, "dimlox: %v\n", err)
	os.Exit(exitCodeFor(err))
}
