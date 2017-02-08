package osfs

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
)

// osFileSystemLoader implements Loader interface using OS file system (os.File).
type osFileSystemLoader struct {
	dirs []string
}

func NewLoader(dirs ...string) *osFileSystemLoader {
	return &osFileSystemLoader{dirs: dirs}
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
