package jet

import (
	"io"
	"os"
	"path"
)

type Loader interface {
	Open(name string) (io.ReadCloser, error)
	Exists(name string) (string, bool)
}

// osFileSystem implements Loader interface with os.Files.
type osFileSystem struct {
	dirs []string
}

// Open opens a file from OS file system.
func (l *osFileSystem) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

// Exists checks if the template name exists by walking the list of template paths
// returns string with the full path of the template and bool true if the template file was found
func (l *osFileSystem) Exists(name string) (string, bool) {
	for i := 0; i < len(l.dirs); i++ {
		fileName := path.Join(l.dirs[i], name)
		if _, err := os.Stat(fileName); err == nil {
			return fileName, true
		}
	}
	return "", false
}
