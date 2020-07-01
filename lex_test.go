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

import "testing"

func lexerTestCaseCustomLexer(t *testing.T, lexer *lexer, input string, items ...itemType) {
	t.Helper()
	for i := 0; i < len(items); i++ {
		item := lexer.nextItem()

		for item.typ == itemSpace {
			item = lexer.nextItem()
		}

		if item.typ != items[i] {
			t.Errorf("Unexpected token %s on input on %q => %q", item, input, input[item.pos:])
			return
		}
	}
	item := lexer.nextItem()
	if item.typ != itemEOF {
		t.Errorf("Unexpected token %s, expected EOF", item)
	}
}

func lexerTestCase(t *testing.T, input string, items ...itemType) {
	lexer := lex("test.flowRender", input, true)
	lexerTestCaseCustomLexer(t, lexer, input, items...)
}

func lexerTestCaseCustomDelimiters(t *testing.T, leftDelim, rightDelim, input string, items ...itemType) {
	lexer := lex("test.customDelimiters", input, false)
	lexer.setDelimiters(leftDelim, rightDelim)
	lexer.run()
	lexerTestCaseCustomLexer(t, lexer, input, items...)
}

func TestLexer(t *testing.T) {
	lexerTestCase(t, `{{}}`, itemLeftDelim, itemRightDelim)
	lexerTestCase(t, `{{ line }}`, itemLeftDelim, itemIdentifier, itemRightDelim)
	lexerTestCase(t, `{{ . }}`, itemLeftDelim, itemIdentifier, itemRightDelim)
	lexerTestCase(t, `{{ .Field }}`, itemLeftDelim, itemField, itemRightDelim)
	lexerTestCase(t, `{{ "value" }}`, itemLeftDelim, itemString, itemRightDelim)
	lexerTestCase(t, `{{ call: value }}`, itemLeftDelim, itemIdentifier, itemColon, itemIdentifier, itemRightDelim)
	lexerTestCase(t, `{{.Ex+1}}`, itemLeftDelim, itemField, itemAdd, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{.Ex-1}}`, itemLeftDelim, itemField, itemMinus, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{.Ex*1}}`, itemLeftDelim, itemField, itemMul, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{.Ex/1}}`, itemLeftDelim, itemField, itemDiv, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{.Ex%1}}`, itemLeftDelim, itemField, itemMod, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{.Ex=1}}`, itemLeftDelim, itemField, itemAssign, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{Ex:=1}}`, itemLeftDelim, itemIdentifier, itemAssign, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{.Ex!1}}`, itemLeftDelim, itemField, itemNot, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{.Ex==1}}`, itemLeftDelim, itemField, itemEquals, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{.Ex&&1}}`, itemLeftDelim, itemField, itemAnd, itemNumber, itemRightDelim)
}

func TestCustomDelimiters(t *testing.T) {
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[]]`, itemLeftDelim, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[ line ]]`, itemLeftDelim, itemIdentifier, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[ . ]]`, itemLeftDelim, itemIdentifier, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[ .Field ]]`, itemLeftDelim, itemField, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[ "value" ]]`, itemLeftDelim, itemString, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[ call: value ]]`, itemLeftDelim, itemIdentifier, itemColon, itemIdentifier, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex+1]]`, itemLeftDelim, itemField, itemAdd, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex-1]]`, itemLeftDelim, itemField, itemMinus, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex*1]]`, itemLeftDelim, itemField, itemMul, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex/1]]`, itemLeftDelim, itemField, itemDiv, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex%1]]`, itemLeftDelim, itemField, itemMod, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex=1]]`, itemLeftDelim, itemField, itemAssign, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[Ex:=1]]`, itemLeftDelim, itemIdentifier, itemAssign, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex!1]]`, itemLeftDelim, itemField, itemNot, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex==1]]`, itemLeftDelim, itemField, itemEquals, itemNumber, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[.Ex&&1]]`, itemLeftDelim, itemField, itemAnd, itemNumber, itemRightDelim)
}

func TestLexNegatives(t *testing.T) {
	lexerTestCase(t, `{{ -1 }}`, itemLeftDelim, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{ 5 + -1 }}`, itemLeftDelim, itemNumber, itemAdd, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{ 5 * -1 }}`, itemLeftDelim, itemNumber, itemMul, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{ 5 / +1 }}`, itemLeftDelim, itemNumber, itemDiv, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{ 5 % -1 }}`, itemLeftDelim, itemNumber, itemMod, itemNumber, itemRightDelim)
	lexerTestCase(t, `{{ 5 == -1000 }}`, itemLeftDelim, itemNumber, itemEquals, itemNumber, itemRightDelim)

}

func TestLexer_Bug35(t *testing.T) {
	lexerTestCase(t, `{{if x>y}}blahblah...{{end}}`, itemLeftDelim, itemIf, itemIdentifier, itemGreat, itemIdentifier, itemRightDelim, itemText, itemLeftDelim, itemEnd, itemRightDelim)
	lexerTestCaseCustomDelimiters(t, "[[", "]]", `[[if x>y]]blahblah...[[end]]`, itemLeftDelim, itemIf, itemIdentifier, itemGreat, itemIdentifier, itemRightDelim, itemText, itemLeftDelim, itemEnd, itemRightDelim)
}
