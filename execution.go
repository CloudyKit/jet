package jet

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
)

var (
	ErrStepLimit      = errors.New("jet: execution step limit exceeded")
	ErrDepthLimit     = errors.New("jet: execution depth limit exceeded")
	ErrInvalidOptions = errors.New("jet: invalid execution options")
)

// ExecutionOptions limits interpreter work in ExecuteContext. Zero is unlimited.
// Limits do not bound allocation, compilation, output, or arbitrary Go callbacks.
type ExecutionOptions struct {
	MaxSteps uint64
	MaxDepth int
}

// LimitError identifies the exhausted allowance. Step totals are implementation
// details, not stable billing units. Depth counts active interpreter frames.
type LimitError struct {
	Kind  string
	Limit uint64
	cause error
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("jet: %s limit exceeded (%d)", e.Kind, e.Limit)
}
func (e *LimitError) Unwrap() error {
	if e.cause != nil {
		return e.cause
	}
	if e.Kind == "depth" {
		return ErrDepthLimit
	}
	return ErrStepLimit
}

// executionState adapts the existing Runtime/Arguments callback APIs to one
// synchronous render. It is never cached and is cleared before pooling Runtime.
type executionState struct {
	ctx       context.Context
	options   ExecutionOptions
	remaining uint64
	depth     int
	terminal  error
}

type executionAbort struct{ error }

// terminate latches the first terminal failure even if a callback recovers its
// panic. Every interpreter recovery and native call return checks the latch.
func (st *Runtime) terminate(err error) {
	if st.execution.terminal == nil {
		st.execution.terminal = err
	}
	panic(executionAbort{st.execution.terminal})
}

func (st *Runtime) rethrowTerminal() {
	if st.execution.terminal != nil {
		panic(executionAbort{st.execution.terminal})
	}
	if st.escapeeWriter != nil && st.writeErr != nil {
		st.terminate(st.writeErr)
	}
}

func (st *Runtime) rethrowRecovered(recovered interface{}) {
	if st.execution.terminal != nil || st.writeErr != nil {
		if _, ok := recovered.(runtime.Error); ok {
			panic(recovered)
		}
		if recovered != nil {
			if _, ok := recovered.(error); !ok {
				panic(recovered)
			}
		}
	}
	st.rethrowTerminal()
}

// checkpoint reserves work before it happens, using subtraction to avoid wrap.
// Additional budget dimensions must reserve against this same execution state.
func (st *Runtime) checkpoint(steps uint64) {
	st.rethrowTerminal()
	b := &st.execution
	if b.ctx == nil {
		return
	}
	if b.ctx.Err() != nil {
		st.terminate(executionContextError(b.ctx))
	}
	st.reserveUnchecked(&b.remaining, b.options.MaxSteps, steps, "steps", ErrStepLimit)
}

// reserve checks an allowance before work/allocation. Zero limit is unlimited;
// failed reservations leave the balance unchanged and terminate the render.
func (st *Runtime) reserve(remaining *uint64, limit, requested uint64, kind string, cause error) {
	st.rethrowTerminal()
	st.reserveUnchecked(remaining, limit, requested, kind, cause)
}

func (st *Runtime) reserveUnchecked(remaining *uint64, limit, requested uint64, kind string, cause error) {
	if limit == 0 {
		return
	}
	if requested > *remaining {
		st.terminate(&LimitError{Kind: kind, Limit: limit, cause: cause})
	}
	*remaining -= requested
}

func (st *Runtime) enter() {
	st.checkpoint(1)
	b := &st.execution
	if b.options.MaxDepth != 0 {
		if b.depth >= b.options.MaxDepth {
			st.terminate(&LimitError{Kind: "depth", Limit: uint64(b.options.MaxDepth)})
		}
		b.depth++
	}
}

func (st *Runtime) leave() {
	if st.execution.options.MaxDepth != 0 {
		st.execution.depth--
	}
}

// executionContext bridges runtime-triggered compilation to the active render.
// Legacy execution has no caller context. No request state belongs on Set/cache.
func (st *Runtime) executionContext() context.Context {
	if st.execution.ctx != nil {
		return st.execution.ctx
	}
	return context.Background()
}

// beforeCall is the common admission point for explicit native invocations.
// Restricted capability admission extends this point; unrestricted calls remain
// synchronous and cannot be preempted while they own execution.
func (st *Runtime) beforeCall(value reflect.Value) { st.checkpoint(1) }

func (st *Runtime) templateRoot(t *Template) *ListNode {
	for t.extends != nil {
		st.checkpoint(1)
		t = t.extends
	}
	st.checkpoint(0)
	return t.Root
}

func (st *Runtime) loadTemplate(name, parent string, sibling bool) (*Template, error) {
	st.checkpoint(1)
	var t *Template
	var err error
	if sibling {
		t, err = st.set.getSiblingTemplate(name, parent, true)
	} else {
		t, err = st.set.GetTemplate(name)
	}
	st.checkpoint(0)
	return t, err
}
