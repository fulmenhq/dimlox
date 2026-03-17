// Package appctx defines context keys and accessors for per-process values
// (logger, etc.) shared across dimlox commands. It is the single place where
// context keys are defined — import this package to read or write those values.
package appctx

import (
	"context"

	"github.com/fulmenhq/gofulmen/logging"
)

type contextKey int

const loggerKey contextKey = iota

// WithLogger returns a copy of ctx carrying log.
func WithLogger(ctx context.Context, log *logging.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// Logger returns the logger stored in ctx, or nil if none is set.
// Commands that need a logger should call this; main() guarantees one is set.
func Logger(ctx context.Context) *logging.Logger {
	log, _ := ctx.Value(loggerKey).(*logging.Logger)
	return log
}
