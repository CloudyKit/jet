package httpfs

import (
	"io"
	"net/http"
	"os"

	"github.com/CloudyKit/jet/v3"
)

type httpFileSystemLoader struct {
	fs http.FileSystem
}

// NewLoader returns an initialized loader serving the passed http.FileSystem.
func NewLoader(fs http.FileSystem) jet.Loader {
	return &httpFileSystemLoader{fs: fs}
}

// Open opens the file via the internal http.FileSystem. It is the callers duty to close the file.
func (l *httpFileSystemLoader) Open(name string) (io.ReadCloser, error) {
	if l.fs == nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	return l.fs.Open(name)
}

// Exists checks if the template name exists by walking the list of template paths
// returns string with the full path of the template and bool true if the template file was found
func (l *httpFileSystemLoader) Exists(name string) (string, bool) {
	if l.fs == nil {
		return "", false
	}
	if f, err := l.Open(name); err == nil {
		f.Close()
		return name, true
	}
	return "", false
}
