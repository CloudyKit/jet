//go:build go1.20
// +build go1.20

package jet

import (
	"context"
	"errors"
)

type contextCauseError struct{ err, cause error }

func (e *contextCauseError) Error() string { return e.err.Error() + ": " + e.cause.Error() }
func (e *contextCauseError) Unwrap() error { return e.err }
func (e *contextCauseError) Is(target error) bool {
	return errors.Is(e.err, target) || errors.Is(e.cause, target)
}

func executionContextError(ctx context.Context) error {
	err, cause := ctx.Err(), context.Cause(ctx)
	if cause != nil && cause != err {
		return &contextCauseError{err: err, cause: cause}
	}
	return err
}
