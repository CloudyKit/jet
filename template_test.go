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
	"reflect"
	"testing"
)

func TestSetSetExtensions(t *testing.T) {
	tests := [][]string{
		{},
		{".html.jet", ".jet"},
		{".tmpl", ".html"},
	}

	for _, extensions := range tests {
		set := &Set{}
		set.SetExtensions(extensions)
		if !reflect.DeepEqual(extensions, set.extensions) {
			t.Errorf("expected extensions %v, got %v", extensions, set.extensions)
		}
	}
}
