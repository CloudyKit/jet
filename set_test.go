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
	"fmt"
	"reflect"
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
