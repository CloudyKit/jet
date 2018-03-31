package main

import (
	"os"
	"fmt"
	"flag"
	"bytes"
	"errors"

	"github.com/CloudyKit/jet"
)

// render renders a template from a given directory name and template name
// (relative to the directory name)
//
// behaves like jet.NewHTMLSet(directory).getTemplate(templateName)
// (plus execution and error handling)
func render(directory string, templateName string) (string, error) {
	view := jet.NewHTMLSet(directory)
	tpl, err := view.GetTemplate(templateName)
	if err != nil {
		return "", err
	}
	var ret bytes.Buffer
	err = tpl.Execute(&ret, nil, nil)
	if err != nil {
		return "", err
	}
	return ret.String(), nil
}

// parses os.Args into a directory and template name and possible error code
func parseArgs() (string, string, error) {
	// set up options, usage, etc
	set := flag.NewFlagSet("", flag.ContinueOnError)
	const templateArg = "template"
	set.Usage = func() {
		argspec := fmt.Sprintf("[options] [-%s] TEMPLATE_NAME", templateArg)
		fmt.Fprintf(set.Output(), "Usage: %s %s\n",
			os.Args[0], argspec)
		set.PrintDefaults()
		println("NOTE: TEMPLATE_NAME can be given either as a named flag or positionally as the last argument")
	}
	directory := set.String("dir", "./", "The directory to search for templates in")
	templateName := set.String(templateArg, "", "The filename of the template to render")

	// parse everything
	err := set.Parse(os.Args[1:])
	if err != nil {
		return "", "", err
	}

	// no -template flag; try the remaining arguments
	if set.Lookup(templateArg).Value.String() == "" {
		// blank template
		if set.NArg() != 1 {
			return "", "", errors.New("Exactly one filename of a template to render required")
		} else {
			// template name given positionally
			// hack to use string as *string
			tmp := set.Arg(0)
			templateName = &tmp
		}
	}
	return *directory, *templateName, err
}

func main() {
	directory, templateName, err := parseArgs()
	if err == flag.ErrHelp {
		// just printed the help msg, not an error
		os.Exit(-1)
	}
	if err != nil {
		os.Stderr.WriteString("Illegal arguments: " + err.Error())
		os.Exit(-1)
	}

	rendered, err := render(directory, templateName)

	if err != nil {
		os.Stderr.WriteString("Jet render error: " + err.Error())
		os.Exit(-1)
	}
	print(rendered)
}
