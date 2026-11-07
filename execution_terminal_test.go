package jet

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestExecutionTerminalLimitsInRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		data         interface{}
		opts         ExecutionOptions
		want         error
	}{
		{"steps", `{{ try }}{{ try }}{{ range ints(0, 1000) }}{{ end }}{{ catch }}caught{{ end }}{{ catch }}caught{{ end }}`, nil, ExecutionOptions{MaxSteps: 100, MaxDepth: 64}, ErrStepLimit},
		{"depth", `{{ try }}{{ try }}{{ block b(n=0) }}{{ if n < 100 }}{{ yield b(n=n+1) }}{{ end }}{{ end }}{{ catch }}caught{{ end }}{{ catch }}caught{{ end }}`, nil, ExecutionOptions{MaxSteps: 10000, MaxDepth: 8}, ErrDepthLimit},
		{"isset depth", `{{ try }}{{ isset(.A) }}{{ catch }}caught{{ end }}`, map[string]int{"A": 1}, ExecutionOptions{MaxSteps: 1000, MaxDepth: 4}, ErrDepthLimit},
		{"content fuel", `{{ block b() }}{{ yield content }}{{ end }}{{ yield b() content }}{{ range ints(0, 1000) }}{{ end }}{{ end }}`, nil, ExecutionOptions{MaxSteps: 100, MaxDepth: 64}, ErrStepLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := runtimeTemplate(t, tc.source).ExecuteContext(context.Background(), &out, nil, tc.data, tc.opts)
			if !errors.Is(err, tc.want) || out.Len() != 0 {
				t.Fatalf("got %v, %q; want %v", err, out.String(), tc.want)
			}
		})
	}
}
