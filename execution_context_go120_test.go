//go:build go1.20
// +build go1.20

package jet

import (
	"context"
	"errors"
	"io/ioutil"
	"testing"
)

func TestExecutionCancellationCause(t *testing.T) {
	cause := errors.New("request cause")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	err := runtimeTemplate(t, "").ExecuteContext(ctx, ioutil.Discard, nil, nil, ExecutionOptions{})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cause) {
		t.Fatal(err)
	}
}
