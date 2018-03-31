package main

import (
	"os"
	"testing"
)

type RenderTestCase struct {
	Args []string
	Expected string
	ArgErr bool
	RenderErr bool
}

// tests rendering
func testRender(t *testing.T, test RenderTestCase) {
	_Args := os.Args
	os.Args = append([]string{"jet"}, test.Args...)
	tpl, dir, err := parseArgs()
	if err != nil {
		if test.ArgErr {
			// expected error
			return
		} else {
			// unexpected error
			t.Errorf("Incorrect argument error for %v; Expected: %v but got %v",
				test.Args, test.ArgErr, err)
		}
	} else if test.ArgErr {
		t.Error("Argument error expected but not found!")
	}
	rendered, err := render(tpl, dir)
	if err != nil {
		if test.RenderErr {
			//expected
			return
		} else {
			t.Errorf("Incorrect render error for %v; Expected: %v but got %v",
				test.Args, test.RenderErr, err)
		}
	} else if test.RenderErr {
		t.Error("Argument error expected but not found!")
	}
	if rendered != test.Expected {
		t.Errorf("Failed for %v; Expected: `%v` but got `%v`",
			test.Args, test.Expected, rendered)
	}
	os.Args = _Args
}

func TestBasic(t *testing.T) {
	testRender(t, RenderTestCase{
		Args: []string{"-dir", "testData", "test.txt"},
		Expected: "title: hello from the jet CLI\nbody:  default body\n",
		ArgErr:    false,
		RenderErr: false,
	})
	testRender(t, RenderTestCase{
		Args: []string{"./testData/test.txt"},
		Expected: "title: hello from the jet CLI\nbody:  default body\n",
		ArgErr:    false,
		RenderErr: false,
	})
	testRender(t, RenderTestCase{
		Args: []string{"./testData/nonexistent"},
		Expected:  "",
		ArgErr:    false,
		RenderErr: true,
	})
	testRender(t, RenderTestCase{
		Args: []string{},
		Expected:  "",
		ArgErr:    true,
		RenderErr: false,
	})
	testRender(t, RenderTestCase{
		Args: []string{"x", "y"},
		Expected:  "",
		ArgErr:    true,
		RenderErr: false,
	})
	testRender(t, RenderTestCase{
		Args: []string{"testData/test/parent.jet"},
		Expected:  "this template contains: a child\n\n",
		ArgErr:    false,
		RenderErr: false,
	})
}

type ArgTestCase struct {
	Args []string
	Template string
	Directory string
	Err bool
}

func testArgs(t *testing.T, test ArgTestCase) {
	_Args := os.Args
	os.Args = append([]string{"jet"}, test.Args...)
	dir, tpl, err := parseArgs()
	if err != nil {
		if !test.Err {
			// unexpected error
			t.Errorf("Unexpected error: Expected none but got %s", err)
		} else {
			// expected error
			return
		}
	}
	if dir != test.Directory {
		t.Errorf("Incorrect directory for %v: expected %s but got %s",
			test.Args, test.Directory, dir)
	}
	if tpl != test.Template {
		t.Errorf("Incorrect template for %v: expected %s but got %s",
			test.Args, test.Template, tpl)
	}
	os.Args = _Args
}

func TestArgs(t *testing.T) {
	testArgs(t, ArgTestCase{
		Args: []string{"-dir", "testData", "test.txt"},
		Template: "test.txt",
		Directory: "testData",
		Err: false,
	})
	testArgs(t, ArgTestCase{
		Args: []string{"-dir", "testData", "-template", "test.txt"},
		Template: "test.txt",
		Directory: "testData",
		Err: false,
	})
	testArgs(t, ArgTestCase{
		Args: []string{"./testData/test.txt"},
		Template: "./testData/test.txt",
		Directory: "./",
		Err: false,
	})
	testArgs(t, ArgTestCase{
		Args: []string{"./testData/nonexistent"},
		Template: "./testData/nonexistent",
		Directory: "./",
		Err: false,
	})
	testArgs(t, ArgTestCase{
		Args: []string{},
		Template: "",
		Directory: "",
		Err: true,
	})
	testArgs(t, ArgTestCase{
		Args: []string{"x", "y"},
		Template: "",
		Directory: "",
		Err: true,
	})
}
