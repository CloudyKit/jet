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
	"testing"
)

// TODO: Add two templates which use includes,
// load to a set, save the parsed result,
// reload the included template with different contents
// compare the new parsed results against the old and
// ensure it changed.
func TestReload(t *testing.T) {

}
