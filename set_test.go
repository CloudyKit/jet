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

package jet

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSetSetExtensions(t *testing.T) {
	tests := [][]string{
		{".html.jet", ".jet"},
		{".tmpl", ".html"},
	}

	for _, extensions := range tests {
		set := NewSet(NewInMemLoader(), WithTemplateNameExtensions(extensions))
		if !reflect.DeepEqual(extensions, set.extensions) {
			t.Errorf("expected extensions %v, got %v", extensions, set.extensions)
		}
	}
}

func TestParseDoesNotCache(t *testing.T) {
	loader := NewInMemLoader()
	set := NewSet(loader)
	_, err := set.Parse("/asd.jet", `{{ foo := "bar" }}{{foo}}`)
	if err != nil {
		t.Fatalf("parsing template: %v", err)
	}
	(set.cache).(*cache).m.Range(func(_, _ interface{}) bool {
		t.Fatalf("template cache is not empty after Parse()")
		return false
	})

	loader.Set("/something_to_extend.jet", "some content to extend")

	_, err = set.Parse("/includes_template.jet", `{{ extends "/something_to_extend.jet" }}, and more content`)
	if err != nil {
		t.Fatalf("parsing template: %v", err)
	}
	(set.cache).(*cache).m.Range(func(_, _ interface{}) bool {
		t.Fatalf("template cache is not empty after Parse()")
		return false
	})
}

func TestGetTemplateConcurrency(t *testing.T) {
	l := NewInMemLoader()
	l.Set("foo", "{{if true}}Hi {{ .Name }}!{{end}}")
	set := NewSet(l)

	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("CC_%d", i), func(t *testing.T) {
			t.Parallel()

			_, err := set.GetTemplate("foo")
			if err != nil {
				t.Errorf("getting template from set: %v", err)
			}
		})
	}
}

// Limit reads so a regression fails without overflowing the process stack.
type cycleTestLoader struct {
	Loader
	reads int
}

func (l *cycleTestLoader) Open(name string) (io.ReadCloser, error) {
	l.reads++
	if l.reads > 8 {
		return nil, fmt.Errorf("too many template reads")
	}
	return l.Loader.Open(name)
}

func TestCyclicTemplateDependencies(t *testing.T) {
	for _, development := range []bool{false, true} {
		for _, tc := range []struct{ name, first, second string }{
			{"self import", `{{import "a.jet"}}`, ""},
			{"mutual import", `{{import "b.jet"}}`, `{{import "a.jet"}}`},
			{"self extends", `{{extends "a.jet"}}`, ""},
			{"mutual extends", `{{extends "b.jet"}}`, `{{extends "a.jet"}}`},
			{"mixed", `{{extends "b.jet"}}`, `{{import "a.jet"}}`},
			{"extension alias", `{{import "b"}}`, `{{import "./a.jet"}}`},
		} {
			t.Run(fmt.Sprintf("%s/development=%t", tc.name, development), func(t *testing.T) {
				memory := NewInMemLoader()
				memory.Set("a.jet", tc.first)
				memory.Set("b.jet", tc.second)
				loader := &cycleTestLoader{Loader: memory}
				set := NewSet(loader, DevelopmentMode(development))
				_, err := set.GetTemplate("a.jet")
				if err == nil || !strings.Contains(err.Error(), "cyclic template dependency") {
					t.Fatalf("expected cyclic dependency error, got %v", err)
				}
			})
		}
	}
}

func TestSharedTemplateDependencies(t *testing.T) {
	for _, development := range []bool{false, true} {
		t.Run(fmt.Sprintf("development=%t", development), func(t *testing.T) {
			loader := NewInMemLoader()
			loader.Set("a.jet", `{{import "b.jet"}}{{import "c.jet"}}{{yield shared()}}`)
			loader.Set("b.jet", `{{import "shared.jet"}}`)
			loader.Set("c.jet", `{{import "shared.jet"}}`)
			loader.Set("shared.jet", `{{block shared()}}ok{{end}}`)
			set := NewSet(loader, DevelopmentMode(development))
			for i := 0; i < 10; i++ {
				t.Run(fmt.Sprint(i), func(t *testing.T) {
					t.Parallel()
					tpl, err := set.GetTemplate("a.jet")
					if err != nil {
						t.Fatal(err)
					}
					var output bytes.Buffer
					if err := tpl.Execute(&output, nil, nil); err != nil {
						t.Fatal(err)
					}
					if output.String() != "ok" {
						t.Fatalf("unexpected output %q", output.String())
					}
				})
			}
		})
	}
}

func TestParseCyclicTemplateDependencies(t *testing.T) {
	memory := NewInMemLoader()
	memory.Set("b.jet", `{{import "a.jet"}}`)
	memory.Set("a.jet", `{{import "b.jet"}}`)
	loader := &cycleTestLoader{Loader: memory}
	set := NewSet(loader)
	if _, err := set.Parse("a.jet", `{{import "b.jet"}}`); err == nil || !strings.Contains(err.Error(), "cyclic template dependency: /a.jet") {
		t.Fatalf("expected cyclic dependency error, got %v", err)
	}
	// A failed parse must not poison later calls on the same set.
	memory.Set("b.jet", "ok")
	if _, err := set.Parse("a.jet", `{{import "b.jet"}}`); err != nil {
		t.Fatal(err)
	}
}
