# Jet CLI

A command-line interface for Jet so you can quickly get started writing Jet
templates. In its simplest case, `jet TEMPLATE` will load and execute the
template named `TEMPLATE` (which might not be in the running directory).

This essentially corresponds to:

    view := jet.NewHTMLSet("./")
    tpl, err := view.GetTemplate(TEMPLATE_NAME)
    var ret bytes.Buffer
    err = tpl.Execute(&ret, nil, nil)
    println(ret.String())

(But there’s some additional error-handling steps; see `jet.go#render(directory,
templateName)` for the implementation.)

# Usage

    Usage: jet [options] [-template] TEMPLATE_NAME
      -dir string
            The directory to search for templates in (default "./")
      -template string
            The filename of the template to render
    NOTE: TEMPLATE_NAME can be given either as a named flag or positionally as
    the last argument

# Build

Build and install with

    go install

from this directory or generate the executable on its own with

    go build

Tests can be run with

    go test
