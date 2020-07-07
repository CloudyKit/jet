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
	"io/ioutil"
	"path"
	"strings"
	"testing"
)

var parseSet = NewSet(nil, "./testData")

type ParserTestCase struct {
	*testing.T
	set *Set
}

func (t ParserTestCase) ExpectPrintName(name, input, output string) {
	set := parseSet
	if t.set != nil {
		set = t.set
	}
	template, err := set.parse(name, input)
	if err != nil {
		t.Errorf("%q %s", input, err.Error())
		return
	}
	expected := strings.Replace(template.String(), "\r\n", "\n", -1)
	output = strings.Replace(output, "\r\n", "\n", -1)
	if expected != output {
		t.Errorf("Unexpected tree on %s Got:\n%s\nExpected: \n%s\n", name, expected, output)
	}
}

func (t ParserTestCase) ExpectPrint(input, output string) {
	t.ExpectPrintName("", input, output)
}

func (t ParserTestCase) ExpectError(name, input, errorMessage string) {
	set := parseSet
	if t.set != nil {
		set = t.set
	}
	_, err := set.parse(name, input)
	if err == nil {
		t.Errorf("expected %q but got no error", errorMessage)
		return
	}
	if err.Error() != errorMessage {
		t.Errorf("expected %q but got %q", errorMessage, err.Error())
	}
}

func (t ParserTestCase) TestPrintFile(file string) {
	content, err := ioutil.ReadFile(path.Join("./testData", file))
	if err != nil {
		t.Errorf("file %s not found", file)
		return
	}
	parts := bytes.Split(content, []byte("==="))
	t.ExpectPrintName(file, string(bytes.TrimSpace(parts[0])), string(bytes.TrimSpace(parts[1])))
}

func (t ParserTestCase) ExpectPrintSame(input string) {
	t.ExpectPrint(input, input)
}

func TestParseTemplateAndImport(t *testing.T) {
	p := ParserTestCase{T: t}
	p.TestPrintFile("extends.jet")
	p.TestPrintFile("imports.jet")
}

func TestUsefulErrorOnLateImportOrExtends(t *testing.T) {
	p := ParserTestCase{T: t}
	p.ExpectError("late_import.jet", `<html><head>{{import "./foo.jet"}}</head></html>`, "template: late_import.jet:1: parsing command: unexpected keyword 'import' ('import' statements must be at the beginning of the template)")
	p.ExpectError("late_extends.jet", `<html><head>{{extends "./foo.jet"}}</head></html>`, "template: late_extends.jet:1: parsing command: unexpected keyword 'extends' ('extends' statements must be at the beginning of the template)")
}

func TestKeywordsDisallowedAsBlockNames(t *testing.T) {
	p := ParserTestCase{T: t}
	p.ExpectError("block_content.jet", `{{ block content() }}bla{{ end }}`, "template: block_content.jet:1: parsing block clause: unexpected keyword 'content' (expected name)")
	p.ExpectError("block_if.jet", `{{ block if() }}bla{{ end }}`, "template: block_if.jet:1: parsing block clause: unexpected keyword 'if' (expected name)")
}

func TestParseTemplateControl(t *testing.T) {
	p := ParserTestCase{T: t}
	p.TestPrintFile("if.jet")
	p.TestPrintFile("range.jet")
}

func TestParseTemplateExpressions(t *testing.T) {
	p := ParserTestCase{T: t}
	p.TestPrintFile("simple_expression.jet")
	p.TestPrintFile("additive_expression.jet")
	p.TestPrintFile("multiplicative_expression.jet")
}

func TestParseTemplateBlockYield(t *testing.T) {
	p := ParserTestCase{T: t}
	p.TestPrintFile("block_yield.jet")
	p.TestPrintFile("new_block_yield.jet")
}

func TestParseTemplateIndexSliceExpression(t *testing.T) {
	p := ParserTestCase{T: t}
	p.TestPrintFile("index_slice_expression.jet")
}

func TestParseTemplateAssignment(t *testing.T) {
	p := ParserTestCase{T: t}
	p.TestPrintFile("assignment.jet")
}

func TestParseTemplateWithCustomDelimiters(t *testing.T) {
	set := NewSet(nil, "./testData")
	set.Delims("[[", "]]")
	p := ParserTestCase{T: t, set: set}
	p.TestPrintFile("custom_delimiters.jet")
}
