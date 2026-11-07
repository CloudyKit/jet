package jet

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"
)

func runtimeTemplate(t *testing.T, source string) *Template {
	t.Helper()
	l := NewInMemLoader()
	l.Set("runtime", source)
	tpl, err := NewSet(l).GetTemplate("runtime")
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

type failingRuntimeWriter struct{ err error }

func (w failingRuntimeWriter) Write(p []byte) (int, error) { return 0, w.err }

func TestRuntimeWriterFailures(t *testing.T) {
	cause := errors.New("sink failed")
	for _, source := range []string{"text", `{{ "<&" }}`, `{{ raw("<&") }}`, `{{ safeHtml("<&") }}`, `{{ json(.) }}`, `{{ try }}text{{ catch }}caught{{ end }}`} {
		t.Run(source, func(t *testing.T) {
			tpl := runtimeTemplate(t, source)
			for _, err := range []error{cause, nil} {
				want := err
				if want == nil {
					want = io.ErrShortWrite
				}
				if got := tpl.Execute(failingRuntimeWriter{err}, nil, "data"); !errors.Is(got, want) {
					t.Fatalf("got %v, want cause %v", got, want)
				}
			}
		})
	}
}

func TestRuntimeReleasesReferences(t *testing.T) {
	for _, source := range []string{`{{ capture() }}`, `{{ capture() }}{{ missing }}`, `{{ capture() }}{{ crash() }}`} {
		t.Run(source, func(t *testing.T) {
			var retained *Runtime
			vars := VarMap{}.SetFunc("capture", func(a Arguments) reflect.Value { retained = a.Runtime(); return reflect.Value{} }).SetFunc("crash", func(a Arguments) reflect.Value { panic("fault") })
			func() {
				defer func() { recover() }()
				_ = runtimeTemplate(t, source).Execute(new(bytes.Buffer), vars, &struct{ Name string }{"caller"})
			}()
			if retained == nil {
				t.Fatal("callback not called")
			}
			if retained.Writer != nil || retained.set != nil || retained.content != nil || retained.context.IsValid() || retained.scope.parent != nil || retained.variables != nil || retained.blocks != nil || retained.includeDepth != 0 {
				t.Fatal("runtime retained caller state")
			}
		})
	}
}

func TestRuntimeTryRestoresContext(t *testing.T) {
	tpl := runtimeTemplate(t, `{{ try }}discard{{ range .Items }}{{ missing }}{{ end }}{{ catch }}{{ .Name }}{{ end }}`)
	var out bytes.Buffer
	if err := tpl.Execute(&out, nil, struct {
		Name  string
		Items []string
	}{"root", []string{"child"}}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "root" {
		t.Fatalf("got %q", out.String())
	}
}
