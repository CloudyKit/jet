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

// +Build ignore
package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/CloudyKit/jet/v3"
)

var views = jet.NewHTMLSet("./views")

type tTODO struct {
	Text string
	Done bool
}

type doneTODOs struct {
	list map[string]*tTODO
	keys []string
	len  int
	i    int
}

func (dt *doneTODOs) New(todos map[string]*tTODO) *doneTODOs {
	dt.len = len(todos)
	for k := range todos {
		dt.keys = append(dt.keys, k)
	}
	dt.list = todos
	return dt
}

// Range satisfies the jet.Ranger interface and only returns TODOs that are done,
// even when the list contains TODOs that are not done.
func (dt *doneTODOs) Range() (reflect.Value, reflect.Value, bool) {
	for dt.i < dt.len {
		key := dt.keys[dt.i]
		dt.i++
		if dt.list[key].Done {
			return reflect.ValueOf(key), reflect.ValueOf(dt.list[key]), false
		}
	}
	return reflect.Value{}, reflect.Value{}, true
}

// Render implements jet.Renderer interface
func (t *tTODO) Render(r *jet.Runtime) {
	done := "yes"
	if !t.Done {
		done = "no"
	}
	r.Write([]byte(fmt.Sprintf("TODO: %s (done: %s)", t.Text, done)))
}

func main() {
	// remove in production
	views.SetDevelopmentMode(true)

	views.AddGlobalFunc("base64", func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("base64", 1, 1)

		buffer := bytes.NewBuffer(nil)
		fmt.Fprint(buffer, a.Get(0))

		return reflect.ValueOf(base64.URLEncoding.EncodeToString(buffer.Bytes()))
	})
	var todos = map[string]*tTODO{
		"example-todo-1": &tTODO{Text: "Add an show todo page to the example project", Done: true},
		"example-todo-2": &tTODO{Text: "Add an add todo page to the example project"},
		"example-todo-3": &tTODO{Text: "Add an update todo page to the example project"},
		"example-todo-4": &tTODO{Text: "Add an delete todo page to the example project", Done: true},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		view, err := views.GetTemplate("todos/index.jet")
		if err != nil {
			log.Println("Unexpected template err:", err.Error())
		}
		view.Execute(w, nil, todos)
	})
	http.HandleFunc("/todo", func(w http.ResponseWriter, r *http.Request) {
		view, err := views.GetTemplate("todos/show.jet")
		if err != nil {
			log.Println("Unexpected template err:", err.Error())
		}
		id := r.URL.Query().Get("id")
		todo, ok := todos[id]
		if !ok {
			http.Redirect(w, r, "/", http.StatusNotFound)
		}
		view.Execute(w, nil, todo)
	})
	http.HandleFunc("/all-done", func(w http.ResponseWriter, r *http.Request) {
		view, err := views.GetTemplate("todos/index.jet")
		if err != nil {
			log.Println("Unexpected template err:", err.Error())
		}
		vars := make(jet.VarMap)
		vars.Set("showingAllDone", true)
		view.Execute(w, vars, (&doneTODOs{}).New(todos))
	})

	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = ":8080"
	} else if !strings.HasPrefix(":", port) {
		port = ":" + port
	}

	log.Println("Serving on " + port)
	http.ListenAndServe(port, nil)
}
