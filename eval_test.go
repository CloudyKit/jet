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
	"runtime"
	"strings"
	"sync"
	"testing"
	"text/template"
)

var (
	JetTestingSet = NewSet(nil)

	ww    io.Writer = (*devNull)(nil)
	users           = []*User{
		{"Mario Santos", "mario@gmail.com"},
		{"Joel Silva", "joelsilva@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Mario Santos", "mario@gmail.com"},
		{"Joel Silva", "joelsilva@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Mario Santos", "mario@gmail.com"},
		{"Joel Silva", "joelsilva@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Mario Santos", "mario@gmail.com"},
		{"Joel Silva", "joelsilva@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Mario Santos", "mario@gmail.com"},
		{"Joel Silva", "joelsilva@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Mario Santos", "mario@gmail.com"},
		{"Joel Silva", "joelsilva@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
	}

	stdSet = template.New("base")
)

type devNull struct{}

func (*devNull) Write(_ []byte) (int, error) {
	return 0, nil
}

func dummy(a string) string {
	return a
}

func init() {
	stdSet.Funcs(template.FuncMap{"dummy": dummy})
	_, err := stdSet.Parse(`
		{{define "actionNode_dummy"}}hello {{dummy "WORLD"}}{{end}}
		{{define "noAllocFn"}}hello {{ "José" }} {{1}} {{ "José" }} {{end}}
		{{define "rangeOverUsers_Set"}}{{range $index,$val := . }}{{$index}}:{{$val.Name}}-{{$val.Email}}{{end}}{{end}}
		{{define "rangeOverUsers"}}{{range . }}{{.Name}}-{{.Email}}{{end}}{{end}}
	`)
	if err != nil {
		println(err.Error())
	}

	JetTestingSet.AddGlobal("dummy", dummy)
	JetTestingSet.LoadTemplate("actionNode_dummy", `hello {{dummy("WORLD")}}`)
	JetTestingSet.LoadTemplate("noAllocFn", `hello {{ "José" }} {{1}} {{ "José" }}`)
	JetTestingSet.LoadTemplate("rangeOverUsers", `{{range .}}{{.Name}}-{{.Email}}{{end}}`)
	JetTestingSet.LoadTemplate("rangeOverUsers_Set", `{{range index,user:= . }}{{index}}{{user.Name}}-{{user.Email}}{{end}}`)

	JetTestingSet.LoadTemplate("BenchNewBlock", "{{ block col(md=12,offset=0) }}\n<div class=\"col-md-{{md}} col-md-offset-{{offset}}\">{{ yield content }}</div>\n\t\t{{ end }}\n\t\t{{ block row(md=12) }}\n<div class=\"row {{md}}\">{{ yield content }}</div>\n\t\t{{ content }}\n<div class=\"col-md-1\"></div>\n<div class=\"col-md-1\"></div>\n<div class=\"col-md-1\"></div>\n\t\t{{ end }}\n\t\t{{ block header() }}\n<div class=\"header\">\n\t{{ yield row() content}}\n\t\t{{ yield col(md=6) content }}\n{{ yield content }}\n\t\t{{end}}\n\t{{end}}\n</div>\n\t\t{{content}}\n<h1>Hey</h1>\n\t\t{{ end }}")
}

func RunJetTest(t *testing.T, variables VarMap, context interface{}, testName, testContent, testExpected string) {
	RunJetTestWithSet(t, JetTestingSet, variables, context, testName, testContent, testExpected)
}

func RunJetTestWithSet(t *testing.T, set *Set, variables VarMap, context interface{}, testName, testContent, testExpected string) {
	var (
		tt  *Template
		err error
	)

	if testContent != "" {
		tt, err = set.LoadTemplate(testName, testContent)
	} else {
		tt, err = set.GetTemplate(testName)
	}

	if err != nil {
		t.Errorf("Parsing error: %s %s %s", err.Error(), testName, testContent)
		return
	}
	RunJetTestWithTemplate(t, tt, variables, context, testExpected)
}

func RunJetTestWithTemplate(t *testing.T, tt *Template, variables VarMap, context interface{}, testExpected string) {
	if testing.RunTests(func(pat, str string) (bool, error) {
		return true, nil
	}, []testing.InternalTest{
		{
			Name: fmt.Sprintf("\tJetTest(%s)", tt.Name),
			F: func(t *testing.T) {
				buff := bytes.NewBuffer(nil)
				err := tt.Execute(buff, variables, context)
				if err != nil {
					t.Errorf("Eval error: %q executing %s", err.Error(), tt.Name)
					return
				}
				result := buff.String()
				if result != testExpected {
					t.Errorf("Result error expected %q got %q on %s", testExpected, result, tt.Name)
				}
			},
		},
	}) == false {
		t.Fail()
	}
}

func TestEvalTextNode(t *testing.T) {
	RunJetTest(t, nil, nil, "textNode", `hello {*Buddy*} World`, `hello  World`)
}

type User struct {
	Name, Email string
}

func (user *User) Format(str string) string {
	return fmt.Sprintf(str, user.Name, user.Email)
}

func (user *User) GetName() string {
	return user.Name
}

func TestEvalActionNode(t *testing.T) {
	var data = make(VarMap)

	data.Set("user", &User{
		"José Santos", "email@example.com",
	})

	RunJetTest(t, nil, nil, "actionNode", `hello {{"world"}}`, `hello world`)
	RunJetTest(t, data, nil, "actionNode_func", `hello {{lower: "WORLD"}}`, `hello world`)
	RunJetTest(t, data, nil, "actionNode_funcPipe", `hello {{lower: "WORLD" |upper}}`, `hello WORLD`)
	RunJetTest(t, data, nil, "actionNode_funcPipeArg", `hello {{lower: "WORLD-" |upper|repeat: 2}}`, `hello WORLD-WORLD-`)
	RunJetTest(t, data, nil, "actionNode_Field", `Oi {{ user.Name }}`, `Oi José Santos`)
	RunJetTest(t, data, nil, "actionNode_Field2", `Oi {{ user.Name }}<{{ user.Email }}>`, `Oi José Santos<email@example.com>`)
	RunJetTest(t, data, nil, "actionNode_Method", `Oi {{ user.Format: "%s<%s>" }}`, `Oi José Santos<email@example.com>`)

	RunJetTest(t, data, nil, "actionNode_Add", `{{ 2+1 }}`, fmt.Sprint(2+1))
	RunJetTest(t, data, nil, "actionNode_Add3", `{{ 2+1+4 }}`, fmt.Sprint(2+1+4))
	RunJetTest(t, data, nil, "actionNode_Add3Minus", `{{ 2+1+4-3 }}`, fmt.Sprint(2+1+4-3))

	RunJetTest(t, data, nil, "actionNode_AddIntString", `{{ 2+"1" }}`, "3")
	RunJetTest(t, data, nil, "actionNode_AddStringInt", `{{ "1"+2 }}`, "12")

	RunJetTest(t, data, nil, "actionNode_NumberNegative", `{{ -5 }}`, "-5")
	RunJetTest(t, data, nil, "actionNode_NumberNegative_1", `{{ 1 + -5 }}`, fmt.Sprint(1+-5))

	//this is an error RunJetTest(t, data, nil, "actionNode_AddStringInt", `{{ "1"-2 }}`, "12")

	RunJetTest(t, data, nil, "actionNode_Mult", `{{ 4*4 }}`, fmt.Sprint(4*4))
	RunJetTest(t, data, nil, "actionNode_MultAdd", `{{ 2+4*4 }}`, fmt.Sprint(2+4*4))
	RunJetTest(t, data, nil, "actionNode_MultAdd1", `{{ 4*2+4 }}`, fmt.Sprint(4*2+4))
	RunJetTest(t, data, nil, "actionNode_MultAdd2", `{{ 2+4*2+4 }}`, fmt.Sprint(2+4*2+4))
	RunJetTest(t, data, nil, "actionNode_MultFloat", `{{ 1.23*1 }}`, fmt.Sprint(1*1.23))
	RunJetTest(t, data, nil, "actionNode_Mod", `{{ 3%2 }}`, fmt.Sprint(3%2))
	RunJetTest(t, data, nil, "actionNode_MultMod", `{{ (1*3)%2 }}`, fmt.Sprint((1*3)%2))
	RunJetTest(t, data, nil, "actionNode_MultDivMod", `{{ (2*5)/ 3 %1 }}`, fmt.Sprint((2*5)/3%1))

	RunJetTest(t, data, nil, "actionNode_Comparation", `{{ (2*5)==10 }}`, fmt.Sprint((2*5) == 10))
	RunJetTest(t, data, nil, "actionNode_Comparatation2", `{{ (2*5)==5 }}`, fmt.Sprint((2*5) == 5))
	RunJetTest(t, data, nil, "actionNode_Logical", `{{ (2*5)==5 || true }}`, fmt.Sprint((2*5) == 5 || true))
	RunJetTest(t, data, nil, "actionNode_Logical2", `{{ (2*5)==5 || false }}`, fmt.Sprint((2*5) == 5 || false))

	RunJetTest(t, data, nil, "actionNode_NumericCmp", `{{ 5*5 > 2*12.5 }}`, fmt.Sprint(5*5 > 2*12.5))
	RunJetTest(t, data, nil, "actionNode_NumericCmp1", `{{ 5*5 >= 2*12.5 }}`, fmt.Sprint(5*5 >= 2*12.5))
	RunJetTest(t, data, nil, "actionNode_NumericCmp1", `{{ 5 * 5 > 2 * 12.5 == 5 * 5 > 2 * 12.5 }}`, fmt.Sprint((5*5 > 2*12.5) == (5*5 > 2*12.5)))
}

func TestEvalIfNode(t *testing.T) {
	var data = make(VarMap)
	data.Set("lower", strings.ToLower)
	data.Set("upper", strings.ToUpper)
	data.Set("repeat", strings.Repeat)

	data.Set("user", &User{
		"José Santos", "email@example.com",
	})

	RunJetTest(t, data, nil, "ifNode_simples", `{{if true}}hello{{end}}`, `hello`)
	RunJetTest(t, data, nil, "ifNode_else", `{{if false}}hello{{else}}world{{end}}`, `world`)
	RunJetTest(t, data, nil, "ifNode_elseif", `{{if false}}hello{{else if true}}world{{end}}`, `world`)
	RunJetTest(t, data, nil, "ifNode_elseif_else", `{{if false}}hello{{else if false}}world{{else}}buddy{{end}}`, `buddy`)
	RunJetTest(t, data, nil, "ifNode_string_comparison", `{{user.Name}} (email: {{user.Email}}): {{if user.Email == "email2@example.com"}}email is email2@example.com{{else}}email is not email2@example.com{{end}}`, `José Santos (email: email@example.com): email is not email2@example.com`)

}

func TestEvalBlockYieldIncludeNode(t *testing.T) {
	var data = make(VarMap)

	data.Set("user", &User{
		"José Santos", "email@example.com",
	})

	RunJetTest(t, data, nil, "Block_simple", `{{block hello() "Buddy" }}Hello {{ . }}{{end}},{{yield hello() user.Name}}`, `Hello Buddy,Hello José Santos`)
	RunJetTest(t, data, nil, "Block_Extends", `{{extends "Block_simple"}}{{block hello() "Buddy" }}Hey {{ . }}{{end}}`, `Hey Buddy,Hey José Santos`)
	RunJetTest(t, data, nil, "Block_Import", `{{import "Block_simple"}}{{yield hello() "Buddy"}}`, `Hello Buddy`)
	RunJetTest(t, data, nil, "Block_Import", `{{import "Block_simple"}}{{yield hello() "Buddy"}}`, `Hello Buddy`)

	JetTestingSet.LoadTemplate("Block_ImportInclude1", `{{yield hello() "Buddy"}}`)
	RunJetTest(t, data, nil, "Block_ImportInclude", `{{ import "Block_simple"}}{{include "Block_ImportInclude1"}}`, `Hello Buddy`)
	RunJetTest(t, data, nil,
		"Block_Content",
		"{{ block col(md=12,offset=0) }}\n<div class=\"col-md-{{md}} col-md-offset-{{offset}}\">{{ yield content }}</div>\n\t\t{{ end }}\n\t\t{{ block row(md=12) }}\n<div class=\"row {{md}}\">{{ yield content }}</div>\n\t\t{{ content }}\n<div class=\"col-md-1\"></div>\n<div class=\"col-md-1\"></div>\n<div class=\"col-md-1\"></div>\n\t\t{{ end }}\n\t\t{{ block header() }}\n<div class=\"header\">\n\t{{ yield row() content}}\n\t\t{{ yield col(md=6) content }}\n{{ yield content }}\n\t\t{{end}}\n\t{{end}}\n</div>\n\t\t{{content}}\n<h1>Hey</h1>\n\t\t{{ end }}",
		"\n<div class=\"col-md-12 col-md-offset-0\"></div>\n\t\t\n\t\t\n<div class=\"row 12\">\n<div class=\"col-md-1\"></div>\n<div class=\"col-md-1\"></div>\n<div class=\"col-md-1\"></div>\n\t\t</div>\n\t\t\n\t\t\n<div class=\"header\">\n\t\n<div class=\"row 12\">\n\t\t\n<div class=\"col-md-6 col-md-offset-0\">\n\n<h1>Hey</h1>\n\t\t\n\t\t</div>\n\t\t\n\t</div>\n\t\t\n</div>\n\t\t",
	)

	JetTestingSet.LoadTemplate("BlockContentLib", "{{block col(columns)}}\n    <div class=\"col {{columns}}\">{{yield content}}</div>\n{{end}}\n{{block row(cols=\"\")}}\n    <div class=\"row\">\n        {{if len(cols) > 0}}\n            {{yield col(columns=cols) content}}{{yield content}}{{end}}\n        {{else}}\n            {{yield content}}\n        {{end}}\n    </div>\n{{end}}")
	RunJetTest(t, nil, nil, "BlockContentParam",
		`{{import "BlockContentLib"}}{{yield row(cols="12") content}}{{cols}}{{end}}`,
		"\n    <div class=\"row\">\n        \n            \n    <div class=\"col 12\">12</div>\n\n        \n    </div>\n")

}

func TestEvalRangeNode(t *testing.T) {

	var data = make(VarMap)

	data.Set("users", []User{
		{"Mario Santos", "mario@gmail.com"},
		{"Joel Silva", "joelsilva@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
	})

	const resultString = `<h1>Mario Santos<small>mario@gmail.com</small></h1><h1>Joel Silva<small>joelsilva@gmail.com</small></h1><h1>Luis Santana<small>luis.santana@gmail.com</small></h1>`
	RunJetTest(t, data, nil, "Range_Expression", `{{range users}}<h1>{{.Name}}<small>{{.Email}}</small></h1>{{end}}`, resultString)
	RunJetTest(t, data, nil, "Range_ExpressionValue", `{{range user:=users}}<h1>{{user.Name}}<small>{{user.Email}}</small></h1>{{end}}`, resultString)
	var resultString2 = `<h1>0: Mario Santos<small>mario@gmail.com</small></h1><h1>Joel Silva<small>joelsilva@gmail.com</small></h1><h1>2: Luis Santana<small>luis.santana@gmail.com</small></h1>`
	RunJetTest(t, data, nil, "Range_ExpressionValueIf", `{{range i, user:=users}}<h1>{{if i == 0 || i == 2}}{{i}}: {{end}}{{user.Name}}<small>{{user.Email}}</small></h1>{{end}}`, resultString2)
}

func TestEvalDefaultFuncs(t *testing.T) {
	RunJetTest(t, nil, nil, "DefaultFuncs_safeHtml", `<h1>{{"<h1>Hello Buddy!</h1>" |safeHtml}}</h1>`, `<h1>&lt;h1&gt;Hello Buddy!&lt;/h1&gt;</h1>`)
	RunJetTest(t, nil, nil, "DefaultFuncs_safeHtml2", `<h1>{{safeHtml: "<h1>Hello Buddy!</h1>"}}</h1>`, `<h1>&lt;h1&gt;Hello Buddy!&lt;/h1&gt;</h1>`)
	RunJetTest(t, nil, nil, "DefaultFuncs_htmlEscape", `<h1>{{html: "<h1>Hello Buddy!</h1>"}}</h1>`, `<h1>&lt;h1&gt;Hello Buddy!&lt;/h1&gt;</h1>`)
	RunJetTest(t, nil, nil, "DefaultFuncs_urlEscape", `<h1>{{url: "<h1>Hello Buddy!</h1>"}}</h1>`, `<h1>%3Ch1%3EHello+Buddy%21%3C%2Fh1%3E</h1>`)

	RunJetTest(t, nil, &User{"Mario Santos", "mario@gmail.com"}, "DefaultFuncs_json", `{{. |writeJson}}`, "{\"Name\":\"Mario Santos\",\"Email\":\"mario@gmail.com\"}\n")

	RunJetTest(t, nil, nil, "DefaultFuncs_replace", `{{replace("My Name Is", " ", "_", -1)}}`, "My_Name_Is")
}

func TestEvalIssetAndTernaryExpression(t *testing.T) {
	var data = make(VarMap)
	data.Set("title", "title")
	RunJetTest(t, nil, nil, "IssetExpression_1", `{{isset(value)}}`, "false")
	RunJetTest(t, data, nil, "IssetExpression_2", `{{isset(title)}}`, "true")
	user := &User{
		"José Santos", "email@example.com",
	}
	RunJetTest(t, nil, user, "IssetExpression_3", `{{isset(.Name)}}`, "true")
	RunJetTest(t, nil, user, "IssetExpression_4", `{{isset(.Names)}}`, "false")
	RunJetTest(t, data, user, "IssetExpression_5", `{{isset(title)}}`, "true")
	RunJetTest(t, data, user, "IssetExpression_6", `{{isset(title.Get)}}`, "false")

	RunJetTest(t, nil, user, "TernaryExpression_4", `{{isset(.Names)?"All names":"no names"}}`, "no names")

	RunJetTest(t, nil, user, "TernaryExpression_5", `{{isset(.Name)?"All names":"no names"}}`, "All names")
	RunJetTest(t, data, user, "TernaryExpression_6", `{{ isset(form) ? form.Get("value") : "no form" }}`, "no form")
}

func TestEvalIndexExpression(t *testing.T) {
	RunJetTest(t, nil, []string{"111", "222"}, "IndexExpressionSlice_1", `{{.[1]}}`, `222`)
	RunJetTest(t, nil, map[string]string{"name": "value"}, "IndexExpressionMap_1", `{{.["name"]}}`, "value")
	RunJetTest(t, nil, map[string]string{"name": "value"}, "IndexExpressionMap_2", `{{.["non_existant_key"]}}`, "")
	RunJetTest(t, nil, map[string]string{"name": "value"}, "IndexExpressionMap_3", `{{isset(.["non_existant_key"]) ? "key does exist" : "key does not exist"}}`, "key does not exist")
	RunJetTest(t, nil, map[string]string{"name": "value"}, "IndexExpressionMap_4", `{{if v, ok := .["name"]; ok}}key does exist and has the value '{{v}}'{{else}}key does not exist{{end}}`, "key does exist and has the value 'value'")
	RunJetTest(t, nil, map[string]string{"name": "value"}, "IndexExpressionMap_5", `{{if v, ok := .["non_existant_key"]; ok}}key does exist and has the value '{{v}}'{{else}}key does not exist{{end}}`, "key does not exist")
	RunJetTest(t, nil, map[string]interface{}{"nested": map[string]string{"name": "value"}}, "IndexExpressionMap_6", `{{.["nested"].name}}`, "value")

	vars := make(VarMap)
	vars.Set("nested", map[string]interface{}{"key": "nested", "nested": map[string]interface{}{"nested": map[string]interface{}{"nested": map[string]interface{}{"name": "value", "strings": []string{"hello"}, "arr": []interface{}{"hello"}}}}})

	//RunJetTest(t, vars, nil, "IndexExpressionMap_6", `{{nested.nested.nested.nested.name}}`, "value")
	// todo: this test is failing with race detector enabled, but looks like a bug when running with the race detector enabled
	RunJetTest(t, vars, nil, "IndexExpressionMap_7", `{{nested.nested.nested.nested.strings[0]}}`, "hello")
	RunJetTest(t, vars, nil, "IndexExpressionMap_8", `{{nested.nested.nested.nested.arr[0]}}`, "hello")
	RunJetTest(t, vars, nil, "IndexExpressionMap_8_1", `{{nested.nested.nested.nested["arr"][0]}}`, "hello")
	RunJetTest(t, vars, nil, "IndexExpressionMap_9", `{{nested[nested.key].nested.nested.name}}`, "value")
	RunJetTest(t, vars, nil, "IndexExpressionMap_10", `{{nested["nested"].nested.nested.name}}`, "value")
	RunJetTest(t, vars, nil, "IndexExpressionMap_11", `{{nested.nested.nested["nested"].name}}`, "value")
	RunJetTest(t, vars, nil, "IndexExpressionMap_12", `{{nested.nested.nested["nested"]["strings"][0]}}`, "hello")
	RunJetTest(t, vars, nil, "IndexExpressionMap_13", `{{nested.nested.nested["nested"]["arr"][0]}}`, "hello")
	RunJetTest(t, vars, nil, "IndexExpressionMap_14", `{{nested["nested"].nested["nested"].name}}`, "value")
	RunJetTest(t, vars, nil, "IndexExpressionMap_15", `{{nested["nested"]["nested"].nested.name}}`, "value")
	RunJetTest(t, nil, &User{"José Santos", "email@example.com"}, "IndexExpressionStruct_1", `{{.[0]}}`, "José Santos")
	RunJetTest(t, nil, &User{"José Santos", "email@example.com"}, "IndexExpressionStruct_2", `{{.["Email"]}}`, "email@example.com")
}

func TestEvalSliceExpression(t *testing.T) {
	RunJetTest(t, nil, []string{"111", "222", "333", "444"}, "SliceExpressionSlice_1", `{{range .[1:]}}{{.}}{{end}}`, `222333444`)
	RunJetTest(t, nil, []string{"111", "222", "333", "444"}, "SliceExpressionSlice_2", `{{range .[:2]}}{{.}}{{end}}`, `111222`)
	RunJetTest(t, nil, []string{"111", "222", "333", "444"}, "SliceExpressionSlice_3", `{{range .[:]}}{{.}}{{end}}`, `111222333444`)
	RunJetTest(t, nil, []string{"111", "222", "333", "444"}, "SliceExpressionSlice_4", `{{range .[0:2]}}{{.}}{{end}}`, `111222`)
	RunJetTest(t, nil, []string{"111", "222", "333", "444"}, "SliceExpressionSlice_5", `{{range .[1:2]}}{{.}}{{end}}`, `222`)
	RunJetTest(t, nil, []string{"111", "222", "333", "444"}, "SliceExpressionSlice_6", `{{range .[1:3]}}{{.}}{{end}}`, `222333`)

	RunJetTest(t, nil, []string{"111"}, "SliceExpressionSlice_BugIndex", `{{range k,v:= . }}{{k}}{{end}}`, `0`)
	RunJetTest(t, nil, []string{"111"}, "SliceExpressionSlice_IfLen", `{{if len(.) > 0}}{{.[0]}}{{end}}`, `111`)
}

func TestEvalBuiltinExpression(t *testing.T) {
	var data = make(VarMap)
	RunJetTest(t, data, nil, "LenExpression_1", `{{len("111")}}`, "3")
	RunJetTest(t, data, nil, "LenExpression_2", `{{isset(data)?len(data):0}}`, "0")
	RunJetTest(t, data, []string{"", "", "", ""}, "LenExpression_3", `{{len(.)}}`, "4")
}

func TestEvalAutoescape(t *testing.T) {
	set := NewHTMLSet()
	RunJetTestWithSet(t, set, nil, nil, "Autoescapee_Test1", `<h1>{{"<h1>Hello Buddy!</h1>" }}</h1>`, "<h1>&lt;h1&gt;Hello Buddy!&lt;/h1&gt;</h1>")
	RunJetTestWithSet(t, set, nil, nil, "Autoescapee_Test2", `<h1>{{"<h1>Hello Buddy!</h1>" |unsafe }}</h1>`, "<h1><h1>Hello Buddy!</h1></h1>")
}

func TestFileResolve(t *testing.T) {
	set := NewHTMLSet("./testData/resolve")
	RunJetTestWithSet(t, set, nil, nil, "simple", "", "simple")
	RunJetTestWithSet(t, set, nil, nil, "simple.jet", "", "simple.jet")
	RunJetTestWithSet(t, set, nil, nil, "extension", "", "extension.jet.html")
	RunJetTestWithSet(t, set, nil, nil, "extension.jet.html", "", "extension.jet.html")
	RunJetTestWithSet(t, set, nil, nil, "./sub/subextend", "", "simple - simple.jet - extension.jet.html")
	RunJetTestWithSet(t, set, nil, nil, "./sub/extend", "", "simple - simple.jet - extension.jet.html")
	//for key, _ := range set.templates {
	//	t.Log(key)
	//}
}

func TestIncludeIfNotExists(t *testing.T) {
	set := NewHTMLSet("./testData/includeIfNotExists")
	RunJetTestWithSet(t, set, nil, nil, "existent", "", "Hi, i exist!!")
	RunJetTestWithSet(t, set, nil, nil, "notExistent", "", "")
	RunJetTestWithSet(t, set, nil, nil, "ifIncludeIfExits", "", "Hi, i exist!!\n    Was included!!\n\n\n    Was not included!!\n\n")
	RunJetTestWithSet(t, set, nil, "World", "wcontext", "", "Hi, Buddy!\nHi, World!")

	// Check if includeIfExists helper bubbles up runtime errors of included templates
	tt, err := set.GetTemplate("includeBroken")
	if err != nil {
		t.Error(err)
	}
	buff := bytes.NewBuffer(nil)
	err = tt.Execute(buff, nil, nil)
	if err == nil {
		t.Error("expected includeIfExists helper to fail with a runtime error but got nil")
	}
}

func TestSet_Parse(t *testing.T) {
	set := NewHTMLSet("./testData/resolve")

	var c int64 = 100

	group := &sync.WaitGroup{}
	for i, l := int64(0), c; i < l; i++ {
		(func() {
			template, _ := set.Parse("TestTemplate", `{{extends "sub/extend"}}`)
			RunJetTestWithTemplate(t, template, nil, nil, "simple - simple.jet - extension.jet.html")
			if len(set.templates) > 0 {
				t.Fail()
			}
			group.Add(1)
			runtime.SetFinalizer(template, func(ob interface{}) {
				group.Done()
			})
		})()
	}
	runtime.GC()
	group.Wait()
}

func BenchmarkSimpleAction(b *testing.B) {
	t, _ := JetTestingSet.GetTemplate("actionNode_dummy")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, nil, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkSimpleActionNoAlloc(b *testing.B) {
	t, _ := JetTestingSet.GetTemplate("noAllocFn")
	for i := 0; i < b.N; i++ {
		t.Execute(ww, nil, nil)
	}
}

func BenchmarkRangeSimple(b *testing.B) {
	t, _ := JetTestingSet.GetTemplate("rangeOverUsers")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, nil, &users)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkRangeSimpleSet(b *testing.B) {
	t, _ := JetTestingSet.GetTemplate("rangeOverUsers_Set")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, nil, &users)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkSimpleActionStd(b *testing.B) {
	t := stdSet.Lookup("actionNode_dummy")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkSimpleActionStdNoAlloc(b *testing.B) {
	t := stdSet.Lookup("noAllocFn")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkRangeSimpleStd(b *testing.B) {
	t := stdSet.Lookup("rangeOverUsers")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, &users)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkRangeSimpleSetStd(b *testing.B) {
	t := stdSet.Lookup("rangeOverUsers_Set")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, &users)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkNewBlockYield(b *testing.B) {
	t, _ := JetTestingSet.GetTemplate("BenchNewBlock")
	b.SetParallelism(10000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := t.Execute(ww, nil, nil)
			if err != nil {
				b.Error(err.Error())
			}
		}
	})

}

func BenchmarkDynamicFunc(b *testing.B) {

	var variables = VarMap{}.Set("dummy", dummy)
	t, _ := JetTestingSet.GetTemplate("actionNode_dummy")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, variables, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkJetFunc(b *testing.B) {
	var variables = VarMap{}.SetFunc("dummy", func(a Arguments) reflect.Value {
		return reflect.ValueOf(dummy(a.Get(0).String()))
	})
	t, _ := JetTestingSet.GetTemplate("actionNode_dummy")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ww, variables, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}
