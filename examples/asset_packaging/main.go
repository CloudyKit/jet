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
//go:generate go run assets/generate.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CloudyKit/jet/v3"
	"github.com/CloudyKit/jet/v3/examples/asset_packaging/assets/templates"
	"github.com/CloudyKit/jet/v3/loaders/httpfs"
	"github.com/CloudyKit/jet/v3/loaders/multi"
)

// Initialize the set with both local files as well as the packaged
// views generated with `go generate` during the build step.
var views = jet.NewHTMLSetLoader(multi.NewLoader(
	jet.NewOSFileSystemLoader("./views"),
	httpfs.NewLoader(templates.Assets),
))

var runAndExit = flag.Bool("run-and-exit", false, "Run app, request / and exit (used in tests)")

func main() {
	flag.Parse()

	// remove in production
	views.SetDevelopmentMode(true)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		view, err := views.GetTemplate("index.jet")
		if err != nil {
			w.WriteHeader(503)
			fmt.Fprintf(w, "Unexpected error while parsing template: %+v", err.Error())
			return
		}
		var resp bytes.Buffer
		if err = view.Execute(&resp, nil, nil); err != nil {
			w.WriteHeader(503)
			fmt.Fprintf(w, "Error when executing template: %+v", err.Error())
			return
		}
		w.WriteHeader(200)
		w.Write(resp.Bytes())
	})

	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = ":9090"
	} else if !strings.HasPrefix(":", port) {
		port = ":" + port
	}

	log.Println("Serving on " + port)
	if *runAndExit {
		go http.ListenAndServe(port, nil)
		time.Sleep(1000) // wait for the server to be up
		resp, err := http.Get("http://localhost" + port + "/")
		if err != nil || resp.StatusCode != 200 {
			r, _ := ioutil.ReadAll(resp.Body)
			log.Printf("An error occurred when fetching page: %+v\n\nResponse:\n%+v\n\nStatus code: %v\n", err, string(r), resp.StatusCode)
			os.Exit(1)
		}
		os.Exit(0)
	}

	http.ListenAndServe(port, nil)
}
