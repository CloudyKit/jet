// Copyright 2016 José Santos <henrique_1609@me.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Jet is a fast and dynamic template engine for the Go programming language, set of features
// includes very fast template execution, a dynamic and flexible language, template inheritance, low number of allocations,
// special interfaces to allow even further optimizations.

package jet

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sort"
)

type VarMap map[string]reflect.Value

// SortedKeys returns a sorted slice of VarMap keys
func (scope VarMap) SortedKeys() []string {
	keys := make([]string, 0, len(scope))
	for k := range scope {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (scope VarMap) Set(name string, v interface{}) VarMap {
	scope[name] = reflect.ValueOf(v)
	return scope
}

func (scope VarMap) SetFunc(name string, v Func) VarMap {
	scope[name] = reflect.ValueOf(v)
	return scope
}

func (scope VarMap) SetWriter(name string, v SafeWriter) VarMap {
	scope[name] = reflect.ValueOf(v)
	return scope
}

// Execute executes the template into w.
func (t *Template) Execute(w io.Writer, variables VarMap, data interface{}) (err error) {
	return t.execute(nil, w, variables, data, ExecutionOptions{})
}

// ExecuteContext executes synchronously with cancellation and interpreter limits.
// It checks before/after native calls and waits for them to return. It does not
// interrupt blocked callbacks, writers, loaders, channels, or custom rangers.
// Nil ctx and negative depth are invalid; zero limits are unlimited.
func (t *Template) ExecuteContext(ctx context.Context, w io.Writer, variables VarMap, data interface{}, opts ExecutionOptions) error {
	if ctx == nil || opts.MaxDepth < 0 {
		return fmt.Errorf("%w: context must be non-nil and depth nonnegative", ErrInvalidOptions)
	}
	return t.execute(ctx, w, variables, data, opts)
}

func (t *Template) execute(ctx context.Context, w io.Writer, variables VarMap, data interface{}, opts ExecutionOptions) (err error) {
	st := pool_State.Get().(*Runtime)
	defer st.recover(&err)

	st.execution = executionState{ctx: ctx, options: opts, remaining: opts.MaxSteps}
	st.checkpoint(0)
	st.blocks = t.processedBlocks
	st.variables = variables
	st.set = t.set
	st.writeErr = nil
	st.rootSink = checkedWriter{writer: w, failure: &st.writeErr}
	st.Writer = &st.rootSink

	root := st.templateRoot(t)

	if data != nil {
		st.context = reflect.ValueOf(data)
	}

	st.executeList(root)
	st.checkpoint(0)
	return
}
