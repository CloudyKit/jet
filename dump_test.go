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
	"reflect"
	"strings"
	"testing"
)

func TestDump(t *testing.T) {
	var b bytes.Buffer                                // writer for the template
	tmplt, err := parseSet.GetTemplate("devdump.jet") // the testing template containing dump function
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	// execute template with dummy inputs
	// MAP
	vars := make(VarMap)
	aMap := make(map[string]interface{})
	aMap["aMap-10"] = 10 // only one member, because map is unsorted; test could fail for no apparent reason.
	vars.Set("inputMap", aMap)
	// SLICE
	aSlice := []string{"sliceMember1", "sliceMember2"}
	vars.Set("aSlice", aSlice)

	// prepare dummy context
	ctx := struct {
		Name    string
		Surname string
	}{Name: "John", Surname: "Doe"}

	// execute template
	err = tmplt.Execute(&b, vars, ctx)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	// normalize EOL convention and
	// split outcome to two parts; this is necessary, because the original code
	// was developed on windows (SORRY !!!!)
	aux := strings.ReplaceAll(b.String(), "\r\n", "\n")
	rslt := strings.Split(aux, "===\n")
	if len(rslt) != 2 {
		t.Log("expected to get two parts, did you include separator in the template?")
		t.FailNow()
	}
	//t.Log(rslt[0])
	// compare what we got with what we wanted
	got := strings.Split(rslt[0], "\n")
	want := strings.Split(rslt[1], "\n")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\ngot :%q\nwant:%q\nAS TEXT\ngot\n%swant\n%s", got, want, rslt[0], rslt[1])
	}
}
