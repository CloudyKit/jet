package jet

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestExecuteConcurrency(t *testing.T) {
	l := NewInMemLoader()
	l.Set("foo", "{{if true}}Hi {{ .Name }}!{{end}}")

	set := NewSet(l)

	tpl, err := set.GetTemplate("foo")
	if err != nil {
		t.Errorf("getting template from set: %v", err)
	}

	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("CC_%d", i), func(t *testing.T) {
			t.Parallel()

			err := tpl.Execute(ioutil.Discard, nil, struct{ Name string }{Name: "John"})
			if err != nil {
				t.Errorf("executing template: %v", err)
			}
		})
	}
}
