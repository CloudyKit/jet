package jet

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExecutionLimits(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		vars         VarMap
		opts         ExecutionOptions
		want         error
	}{
		{"empty", "", nil, ExecutionOptions{MaxSteps: 1, MaxDepth: 1}, nil},
		{"literal", "x", nil, ExecutionOptions{MaxSteps: 2, MaxDepth: 1}, nil},
		{"exact steps", "x", nil, ExecutionOptions{MaxSteps: 1}, ErrStepLimit},
		{"expression depth", `{{ 1 + (2 + 3) }}`, nil, ExecutionOptions{MaxDepth: 2}, ErrDepthLimit},
		{"silent", `{{ range ints(0, 100000) }}{{ end }}`, nil, ExecutionOptions{MaxSteps: 100}, ErrStepLimit},
		{"expression", `{{ 1 + (2 + (3 + 4)) }}`, nil, ExecutionOptions{MaxSteps: 5}, ErrStepLimit},
		{"equality", `{{ a == b }}`, VarMap{}.Set("a", make([]int, 1000)).Set("b", make([]int, 1000)), ExecutionOptions{MaxSteps: 100}, ErrStepLimit},
		{"string equality", `{{ a == b }}`, VarMap{}.Set("a", strings.Repeat("x", 1000)).Set("b", strings.Repeat("x", 1000)), ExecutionOptions{MaxSteps: 100}, ErrStepLimit},
		{"array truth", `{{ if a }}yes{{ end }}`, VarMap{}.Set("a", [1000]int{}), ExecutionOptions{MaxSteps: 100}, ErrStepLimit},
		{"max integers", `{{ 1 + 2 }}`, nil, ExecutionOptions{MaxSteps: math.MaxUint64, MaxDepth: math.MaxInt32}, nil},
		{"unlimited", `{{ 1 + 2 }}`, nil, ExecutionOptions{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runtimeTemplate(t, tc.source).ExecuteContext(context.Background(), ioutil.Discard, tc.vars, nil, tc.opts)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if tc.want == ErrStepLimit || tc.want == ErrDepthLimit {
				var limit *LimitError
				if !errors.As(err, &limit) || limit.Limit == 0 {
					t.Fatalf("missing limit details: %v", err)
				}
			}
		})
	}
}

func TestExecutionCyclicValues(t *testing.T) {
	a, b := make(map[string]interface{}), make(map[string]interface{})
	a["self"], b["self"] = a, b
	var pointer interface{}
	pointer = &pointer
	for _, tc := range []struct {
		source string
		vars   VarMap
		opts   ExecutionOptions
		want   error
	}{
		{`{{ a == b }}`, VarMap{}.Set("a", a).Set("b", b), ExecutionOptions{MaxSteps: 100}, ErrStepLimit},
		{`{{ a == b }}`, VarMap{}.Set("a", a).Set("b", b), ExecutionOptions{MaxDepth: 10}, ErrDepthLimit},
		{`{{ p.Field }}`, VarMap{}.Set("p", pointer), ExecutionOptions{MaxSteps: 100}, ErrStepLimit},
		{`{{ range p }}{{ end }}`, VarMap{}.Set("p", pointer), ExecutionOptions{MaxSteps: 100}, ErrStepLimit},
	} {
		t.Run(tc.source, func(t *testing.T) {
			if err := runtimeTemplate(t, tc.source).ExecuteContext(context.Background(), ioutil.Discard, tc.vars, nil, tc.opts); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
}

func TestExecutionIssetCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	vars := VarMap{}.Set("a", map[string]string{"x": "y"}).SetFunc("cancel", func(a Arguments) reflect.Value { cancel(); return reflect.ValueOf("x") })
	var out bytes.Buffer
	err := runtimeTemplate(t, `{{ try }}{{ isset(a[cancel()]) }}{{ catch }}caught{{ end }}`).ExecuteContext(ctx, &out, vars, nil, ExecutionOptions{})
	if !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatalf("got %v, %q", err, out.String())
	}
}

func TestExecutionClearsCancelledState(t *testing.T) {
	var retained *Runtime
	ctx, cancel := context.WithCancel(context.Background())
	vars := VarMap{}.SetFunc("cancel", func(a Arguments) reflect.Value { retained = a.Runtime(); cancel(); return reflect.Value{} })
	if err := runtimeTemplate(t, `{{ cancel() }}`).ExecuteContext(ctx, ioutil.Discard, vars, nil, ExecutionOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if retained.execution.ctx != nil || retained.execution.terminal != nil || retained.execution.depth != 0 || retained.writeErr != nil || retained.Writer != nil || retained.set != nil {
		t.Fatal("execution state retained")
	}
	if err := runtimeTemplate(t, "ok").Execute(ioutil.Discard, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionReservation(t *testing.T) {
	st := &Runtime{escapeeWriter: new(escapeeWriter)}
	remaining := uint64(math.MaxUint64)
	st.reserve(&remaining, math.MaxUint64, math.MaxUint64, "steps", ErrStepLimit)
	if remaining != 0 {
		t.Fatal(remaining)
	}
	defer func() {
		r := recover()
		if r == nil || !errors.Is(st.execution.terminal, ErrStepLimit) || remaining != 0 {
			t.Fatalf("got %v, balance %d", r, remaining)
		}
	}()
	st.reserve(&remaining, math.MaxUint64, 1, "steps", ErrStepLimit)
}

func TestExecutionProgrammingPanic(t *testing.T) {
	for _, source := range []string{`{{ fault() }}`, `{{ try }}{{ fault() }}{{ catch }}caught{{ end }}`} {
		t.Run(source, func(t *testing.T) {
			vars := VarMap{}.SetFunc("fault", func(a Arguments) reflect.Value {
				defer func() { recover(); var pointer *int; _ = *pointer }()
				a.Runtime().terminate(ErrStepLimit)
				return reflect.Value{}
			})
			defer func() {
				if _, ok := recover().(runtime.Error); !ok {
					t.Fatal("programming panic was hidden")
				}
			}()
			_ = runtimeTemplate(t, source).ExecuteContext(context.Background(), ioutil.Discard, vars, nil, ExecutionOptions{})
		})
	}
}

func TestExecutionCompilationContextBridge(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "request")
	vars := VarMap{}.SetFunc("check", func(a Arguments) reflect.Value {
		if a.Runtime().executionContext() != ctx {
			t.Error("lost render context")
		}
		return reflect.Value{}
	})
	if err := runtimeTemplate(t, `{{ check() }}`).ExecuteContext(ctx, ioutil.Discard, vars, nil, ExecutionOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionInvalidOptions(t *testing.T) {
	tpl := runtimeTemplate(t, "")
	for _, tc := range []struct {
		ctx  context.Context
		opts ExecutionOptions
	}{{nil, ExecutionOptions{}}, {context.Background(), ExecutionOptions{MaxDepth: -1}}} {
		if err := tpl.ExecuteContext(tc.ctx, ioutil.Discard, nil, nil, tc.opts); !errors.Is(err, ErrInvalidOptions) {
			t.Fatal(err)
		}
	}
}

func TestExecutionCancelledEntryAndCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, source := range []string{"", "text"} {
		if err := runtimeTemplate(t, source).ExecuteContext(ctx, ioutil.Discard, nil, nil, ExecutionOptions{}); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	expired, stop := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stop()
	if err := runtimeTemplate(t, "").ExecuteContext(expired, ioutil.Discard, nil, nil, ExecutionOptions{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	vars := VarMap{}.SetFunc("cancel", func(a Arguments) reflect.Value { cancel(); return reflect.Value{} })
	if err := runtimeTemplate(t, `{{ cancel() }}`).ExecuteContext(ctx, ioutil.Discard, vars, nil, ExecutionOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestExecutionOwnsActiveCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, exited := make(chan struct{}), make(chan struct{}), make(chan struct{})
	vars := VarMap{}.SetFunc("wait", func(a Arguments) reflect.Value { close(entered); <-release; close(exited); return reflect.Value{} })
	tpl := runtimeTemplate(t, `{{ wait() }}`)
	returned := make(chan error, 1)
	go func() { returned <- tpl.ExecuteContext(ctx, ioutil.Discard, vars, nil, ExecutionOptions{}) }()
	<-entered
	cancel()
	select {
	case err := <-returned:
		t.Errorf("returned while callback active: %v", err)
	default:
	}
	close(release)
	if err := <-returned; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("callback outlived execution")
	}
}

// Err supplies a deterministic cancellation checkpoint during native-only work.
type checkpointContext struct {
	context.Context
	calls, cancelAt int
	returned        bool
}

func (c *checkpointContext) Err() error {
	if c.returned {
		panic("interpreter work after return")
	}
	c.calls++
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestExecutionCancelsSilentNativeWork(t *testing.T) {
	ctx := &checkpointContext{Context: context.Background(), cancelAt: 100}
	err := runtimeTemplate(t, `{{ range ints(0, 100000) }}{{ end }}`).ExecuteContext(ctx, ioutil.Discard, nil, nil, ExecutionOptions{})
	ctx.returned = true
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if ctx.calls > 102 {
		t.Fatalf("unexpected cancellation overshoot: %d", ctx.calls)
	}
}

func TestExecutionTerminalTryAndIsset(t *testing.T) {
	for _, source := range []string{
		`{{ try }}{{ range ints(0, 100000) }}{{ end }}{{ catch }}caught{{ end }}`,
		`{{ try }}{{ try }}{{ block b(n=0) }}{{ if n < 100 }}{{ yield b(n=n+1) }}{{ end }}{{ end }}{{ catch }}caught{{ end }}{{ catch }}caught{{ end }}`,
		`{{ try }}{{ isset(.A.B.C.D) }}{{ catch }}caught{{ end }}`,
	} {
		t.Run(source, func(t *testing.T) {
			var out bytes.Buffer
			err := runtimeTemplate(t, source).ExecuteContext(context.Background(), &out, nil, nil, ExecutionOptions{MaxSteps: 8, MaxDepth: 8})
			if !errors.Is(err, ErrStepLimit) && !errors.Is(err, ErrDepthLimit) {
				t.Fatalf("got %v", err)
			}
			if strings.Contains(out.String(), "caught") {
				t.Fatalf("caught termination: %q", out.String())
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	vars := VarMap{}.SetFunc("cancel", func(a Arguments) reflect.Value { cancel(); return reflect.Value{} })
	var out bytes.Buffer
	err := runtimeTemplate(t, `{{ try }}{{ cancel() }}{{ catch }}caught{{ end }}`).ExecuteContext(ctx, &out, vars, nil, ExecutionOptions{})
	if !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatalf("got %v, %q", err, out.String())
	}
}

func TestExecutionStickyRecoveredTermination(t *testing.T) {
	vars := VarMap{}.SetFunc("swallow", func(a Arguments) (result reflect.Value) {
		defer func() { recover() }()
		for {
			a.Runtime().YieldBlock("b", nil)
		}
	})
	err := runtimeTemplate(t, `{{ block b() }}x{{ end }}{{ swallow() }}`).ExecuteContext(context.Background(), ioutil.Discard, vars, nil, ExecutionOptions{MaxSteps: 100})
	if !errors.Is(err, ErrStepLimit) {
		t.Fatal(err)
	}
}

func TestExecutionCompositionSharesBudget(t *testing.T) {
	cases := []string{
		`{{ include "entry" }}`,
		`{{ exec("entry") }}`,
		`{{ includeIfExists("entry") }}`,
		`{{ block b() }}{{ yield b() }}{{ end }}`,
		`{{ block a(n=0) }}{{ if n < 1 }}{{ yield b() }}{{ end }}{{ end }}{{ block b() }}{{ yield a() }}{{ end }}`,
		`{{ block b() }}{{ reenter() }}{{ end }}`,
		`{{ block b() }}{{ yield content }}{{ end }}{{ yield b() content }}{{ yield b() content }}{{ yield content }}{{ end }}{{ end }}`,
	}
	if source := os.Getenv("JET_EXECUTION_RECURSION"); source != "" {
		debug.SetMaxStack(256 << 10)
		l := NewInMemLoader()
		l.Set("entry", source)
		tpl, err := NewSet(l).GetTemplate("entry")
		if err != nil {
			t.Fatal(err)
		}
		vars := VarMap{}.SetFunc("reenter", func(a Arguments) reflect.Value { a.Runtime().YieldBlock("b", nil); return reflect.Value{} })
		err = tpl.ExecuteContext(context.Background(), ioutil.Discard, vars, nil, ExecutionOptions{MaxDepth: 12, MaxSteps: 1000})
		// Finite content composition must also work with a sufficient allowance.
		if strings.Contains(source, "yield content") {
			if err != nil {
				t.Fatal(err)
			}
			return
		}
		if !errors.Is(err, ErrDepthLimit) {
			t.Fatal(err)
		}
		return
	}
	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestExecutionCompositionSharesBudget$")
			cmd.Env = append(os.Environ(), "JET_EXECUTION_RECURSION="+source)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("bounded child: %v\n%s", err, output)
			}
		})
	}
}

func TestExecutionConcurrentBudgets(t *testing.T) {
	tpl := runtimeTemplate(t, `{{ range ints(0, n) }}{{ end }}ok`)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			var out bytes.Buffer
			err := tpl.ExecuteContext(context.Background(), &out, VarMap{}.Set("n", n), nil, ExecutionOptions{MaxSteps: 100})
			if n < 10 {
				if err != nil || out.String() != "ok" {
					t.Errorf("%d: %v, %q", n, err, out.String())
				}
			} else if !errors.Is(err, ErrStepLimit) {
				t.Errorf("%d: %v", n, err)
			}
		}(1 + i*100)
	}
	wg.Wait()
}
