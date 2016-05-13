// +Build ignore
package main

import (
	"net/http"
	"os"
	"github.com/CloudyKit/jet"
	"log"
)

var views = jet.NewHTMLSet("./views")

type tTODO struct {
	Text string
	Done bool
}

func main() {

	var todos = map[string]*tTODO{
		"add an show todo page":&tTODO{Text:"Add an show todo page to the example project", Done:true},
		"add an add todo page":&tTODO{Text:"Add an add todo page to the example project"},
		"add an update todo page":&tTODO{Text:"Add an update todo page to the example project"},
		"add an delete todo page":&tTODO{Text:"Add an delete todo page to the example project"},
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

