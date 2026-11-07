# Context-aware execution

`Template.ExecuteContext` executes synchronously with cancellation and optional interpreter limits:

```go
err := tpl.ExecuteContext(ctx, writer, vars, data, jet.ExecutionOptions{
    MaxSteps: 100000,
    MaxDepth: 64,
})
if errors.Is(err, jet.ErrStepLimit) || errors.Is(err, jet.ErrDepthLimit) {
    // Discard partial application output or handle the limit failure.
}
```

These are example limits, not safe defaults for arbitrary templates or data. `Execute` remains available with unlimited interpreter work and existing extension points.

| Option | Type | Default | Meaning |
| --- | --- | --- | --- |
| MaxSteps | uint64 | 0 | Interpreter work allowance; zero is unlimited |
| MaxDepth | int | 0 | Active interpreter frames; zero is unlimited; negatives are invalid |

Nil context returns `ErrInvalidOptions` before runtime acquisition. Cancellation is checked at entry and completion, including empty templates, and around interpreter work and native calls. On Go 1.20+, `errors.Is` recognizes both the context error and a custom cancellation cause. Earlier Go versions preserve `ctx.Err()` without raising the module's Go 1.16 requirement.

Work is charged before statements, expressions, loop advances, recursive value traversal and numeric string scans. Native string comparison reserves its scan bound before comparison. Statements in unvisited branches are not charged. Step totals are implementation details, not stable billing units. Depth covers active list, expression, value traversal and composition frames; it is broader than an include count. For example, a text-only template fits depth one, while evaluating an expression requires additional frames. The tests exercise exact boundaries.

Nested includes, `exec`, `includeIfExists`, blocks/content and `Runtime.YieldBlock` retain the same state and allowance. Discarding `exec` output does not reset compute limits. Reentering public Execute/ExecuteContext from arbitrary Go code starts a separate top-level render; this is outside shared nested interpreter accounting.

Cancellation and limit failures are terminal. Template `try/catch` and `isset` cannot convert them to success, including when native code recovers an interpreter panic. `LimitError` exposes `Kind` and `Limit` and unwraps to the corresponding sentinel. Ordinary template errors remain catchable, including try output rollback. Existing public panic behavior is preserved: `runtime.Error` and non-error panics escaping ordinary template recovery are rethrown; a programming fault after recovered termination is not mislabeled a budget failure.

Sink errors and short writes abort the render, even through escaping, SafeWriter callbacks, or try buffer commits. `errors.Is` reaches the original cause or `io.ErrShortWrite`. Earlier committed output may remain on failure. Callers that need atomic output must render into their own buffer and publish it only after success.

Return means the interpreter has stopped using borrowed vars, data and writer. Runtime and ranger pools clear execution references on every unwind. Callers must synchronize mutations during execution. Arbitrary Go functions, custom Ranger/Renderer/SafeWriter implementations, channels, loaders, writers and implicit native formatting hooks cannot be preempted. Execution waits for an active callback to return; it does not start or abandon a timeout worker. Host code retaining arguments or launching independent work has its own lifetime obligations.

This API does not bound value allocation, output generation, compilation/loading, input admission or arbitrary native code. A very large depth allowance can still exceed the available Go stack. Zero limits provide no protection. Execution budgets alone do not establish an untrusted-template sandbox or a process CPU/memory boundary.

Run `go test ./...` for compatibility and `go test -race ./...` separately for supported concurrent usage. `BenchmarkExecutionBudget` compares cached execution with legacy and positive-limit APIs; compilation and application workflows require separate measurements.
