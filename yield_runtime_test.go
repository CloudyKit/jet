package jet

import (
	"bytes"
	"reflect"
	"testing"
)

func TestRuntimeYieldBlockContext(t *testing.T) {
	tpl := runtimeTemplate(t, `{{ block b() }}{{ . }}{{ end }}{{ emit() }}`)
	vars := VarMap{}.SetFunc("emit", func(a Arguments) reflect.Value { a.Runtime().YieldBlock("b", "child"); return reflect.Value{} })
	var out bytes.Buffer
	if err := tpl.Execute(&out, vars, "root"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "rootchild" {
		t.Fatalf("got %q", out.String())
	}
}

func TestRuntimeYieldBlockRestoresAfterPanic(t *testing.T) {
	tpl := runtimeTemplate(t, `{{ block b() }}{{ if . == "child" }}{{ missing }}{{ end }}{{ end }}{{ emit() }}`)
	vars := VarMap{}.SetFunc("emit", func(a Arguments) (result reflect.Value) {
		defer func() { recover(); result = a.Runtime().Context() }()
		a.Runtime().YieldBlock("b", "child")
		return reflect.Value{}
	})
	var out bytes.Buffer
	if err := tpl.Execute(&out, vars, "root"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "root" {
		t.Fatalf("got %q", out.String())
	}
}
