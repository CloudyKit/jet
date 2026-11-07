package jet

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"
)

func BenchmarkExecutionBudget(b *testing.B) {
	for _, fixture := range []struct {
		name, source string
		data         interface{}
	}{
		{"header", `X-Name: {{ . }}`, "sample"},
		{"body8K", `{{ . }}`, strings.Repeat("a", 8192)},
		{"silent", `{{ range ints(0, 100) }}{{ end }}`, nil},
		{"composition", `{{ block b(n=1) }}{{ n }}{{ end }}{{ range ints(0, 10) }}{{ yield b() }}{{ end }}`, nil},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			l := NewInMemLoader()
			l.Set("bench", fixture.source)
			tpl, err := NewSet(l).GetTemplate("bench")
			if err != nil {
				b.Fatal(err)
			}
			for _, metered := range []bool{false, true} {
				name := "legacy"
				if metered {
					name = "metered"
				}
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if metered {
							err = tpl.ExecuteContext(context.Background(), ioutil.Discard, nil, fixture.data, ExecutionOptions{MaxSteps: 1000000, MaxDepth: 128})
						} else {
							err = tpl.Execute(ioutil.Discard, nil, fixture.data)
						}
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
