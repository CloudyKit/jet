//go:build !go1.20
// +build !go1.20

package jet

import "context"

func executionContextError(ctx context.Context) error { return ctx.Err() }
