// +Build ignore
package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/CloudyKit/jet"
	"log"
	"net/http"
	"os"
	"reflect"
)

var views = jet.NewHTMLSet("./views")

type tTODO struct {
	Text string
	Done bool
}

func main() {
	//todo: remove in production
	views.SetDevelopmentMode(true)

	views.AddGlobalFunc("base64", func(a jet.Arguments) reflect.Value {
		a.RequireNumOfArguments("base64", 1, 1)

		buffer := bytes.NewBuffer(nil)
		fmt.Fprint(buffer, a.Get(0))

		return reflect.ValueOf(base64.URLEncoding.EncodeToString(buffer.Bytes()))
	})
	var todos = map[string]*tTODO{
		"add an show todo page":   &tTODO{Text: "Add an show todo page to the example project", Done: true},
		"add an add todo page":    &tTODO{Text: "Add an add todo page to the example project"},
		"add an update todo page": &tTODO{Text: "Add an update todo page to the example project"},
		"add an delete todo page": &tTODO{Text: "Add an delete todo page to the example project"},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		view, err := views.GetTemplate("showtodos.jet")
		if err != nil {
			log.Println("Unexpected template err:", err.Error())
		}
		view.Execute(w, nil, todos)
	})

	http.ListenAndServe(os.Getenv("PORT"), nil)
}
