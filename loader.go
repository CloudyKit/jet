package jet

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
)

// Loader is a minimal interface required for loading templates.
type Loader interface {
	// Open opens the underlying reader with template content.
	Open(name string) (io.ReadCloser, error)
	// Exists checks for template existence and returns full path.
	Exists(name string) (string, bool)
}

// hasAddPath is an optional Loader interface. Most probably useful for OS file system only, thus unexported.
type hasAddPath interface {
	AddPath(path string)
}

// hasAddGopathPath is an optional Loader interface. Most probably useful for OS file system only, thus unexported.
type hasAddGopathPath interface {
	AddGopathPath(path string)
}

// osFileSystemLoader implements Loader interface using OS file system (os.File).
type osFileSystemLoader struct {
	dirs []string
}

// Open opens a file from OS file system.
func (l *osFileSystemLoader) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

// Exists checks if the template name exists by walking the list of template paths
// returns string with the full path of the template and bool true if the template file was found
func (l *osFileSystemLoader) Exists(name string) (string, bool) {
	for i := 0; i < len(l.dirs); i++ {
		fileName := path.Join(l.dirs[i], name)
		if _, err := os.Stat(fileName); err == nil {
			return fileName, true
		}
	}
	return "", false
}

func (l *osFileSystemLoader) AddPath(path string) {
	l.dirs = append(l.dirs, path)
}

func (l *osFileSystemLoader) AddGopathPath(path string) {
	paths := filepath.SplitList(os.Getenv("GOPATH"))
	for i := 0; i < len(paths); i++ {
		path, err := filepath.Abs(filepath.Join(paths[i], "src", path))
		if err != nil {
			panic(errors.New("Can't add this path err: " + err.Error()))
		}

		if fstats, err := os.Stat(path); os.IsNotExist(err) == false && fstats.IsDir() {
			l.AddPath(path)
			return
		}
	}

	if fstats, err := os.Stat(path); os.IsNotExist(err) == false && fstats.IsDir() {
		l.AddPath(path)
	}
}
