# Asset packaging example

This example demonstrates how to package up your templates into your app. This is useful for your web app deployment to only consist of copying over the compiled binary.

To cut down on the complexity of this example packaging up your public folder and other local assets is not shown but the process is easily extensible to incorporate these files and folders as well.

To see this project in action:

```
  $ go get -u "github.com/shurcooL/vfsgen"
  $ make build
```

That will build the app while compiling in the views folder in this directory. To be sure, move the compiled binary to another location, then run it from there. Access http://localhost:9090 in your browser and see that it works regardless of location.

Local development is also possible. Do a `make run` in this directory, change something in the templates, refresh the browser and see it reflected there. This example is set up to use the local files in development as well as having Jet's development mode on which doesn't cache the templates – disabling the development mode when running in production is about the only thing not covered in this example because it'll depend on your app and its configuration on how this is done.

Finally, for anyone looking for a step-by-step guide on how this is accomplished:

1. Add `github.com/shurcooL/vfsgen` to your project (vendoring is encouraged)
2. Add the `assets/generate.go` and `assets/templates/templates.go` files (copy the contents from this project)
3. Add `//go:generate go run assets/generate.go` to your `main.go` file (above `package main`)
4. Add a build target to your Makefile like you see in this project.

Here's the rundown: when the Makefile target executes, it will first run `go generate`. This will look through the Go files in the current directory and search for annotations like you added above: `//go:generate` and run the command there. That runs the asset generation through `vfsgen` and generates the `templates_vfsdata.go` file you see when the build finishes. Through some build tags that are only set on this build (`deploy_build` in this case), only that file is included in the binary and that contains the view files as binary data.

The last thing is to configure the Jet template engine via the multi loader to also use that `http.FileSystem` to look for templates – that's done in the `main.go` file.

This is it, the templates are now loaded from within the binary. This process can be extended to include more directory trees – just add another folder to the assets directory, configure vfsgen in the generate.go file to fetch that directory tree and you're done. We did that in our projects with locale files as well as the whole public folder. As Gophers like to say: Just One Binary™.
