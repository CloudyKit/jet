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
	"sort"
	"strings"
	"testing"
)

func TestDump(t *testing.T) {
	var b bytes.Buffer
	tmplt, err := parseSet.GetTemplate("devdump.jet")
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	// dump does NOT dump inputs to the template
	// inputs := make(VarMap)
	// inputs["input1"] = reflect.ValueOf("input1")
	// inputs["input2"] = reflect.ValueOf(10)

	// dump does NOT dump data
	// data := make(map[string]string)
	// data["d1"] = "val1"
	// data["d2"] = "val2"

	// execute template
	err = tmplt.Execute(&b, nil, nil)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	// split outcome to two parts
	rslt := strings.Split(b.String(), "===")
	if len(rslt) != 2 {
		t.Log("expected to get two parts, did you include separator in the template?")
		t.FailNow()
	}
	got := newTstKeyValList(rslt[0])
	want := newTstKeyValList(rslt[1])
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got:%q, want:%q", got, want)
	}
}

// tstKeyValList implements sort on the map
type tstKeyValList []tstKeyValPair

func newTstKeyValList(in string) tstKeyValList {
	lines := strings.Split(in, "\n")
	kv := make(tstKeyValList, 0, len(lines))
	for _, line := range lines {
		pair := newKeyVal(line)
		if pair.key == "" || pair.val == "" {
			continue // do not compare lines without equal sign
		}
		kv = append(kv, pair)
	}
	kv.Sort()
	return kv
}

func newKeyVal(line string) tstKeyValPair {
	ret := tstKeyValPair{}
	aux := strings.Split(line, "=")
	if len(aux) != 2 {
		return ret
	}
	ret.key = aux[0]
	ret.val = aux[1]
	return ret
}

// tstKeyValPair is a key value pair
type tstKeyValPair struct {
	key string
	val string
}

// Sort implements sort by keys
func (l *tstKeyValList) Sort() {
	sort.Sort(l)
}

// Len is a part of sort.Interface.
func (l tstKeyValList) Len() int { return len(l) }

// Less is a part of sort.Interface.
func (l tstKeyValList) Less(i, j int) bool { return string(l[i].key) < string(l[j].key) }

// Swap is a part of sort.Interface.
func (l tstKeyValList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
