package httpfs

import (
	"errors"
	"io"
	"net/http"

	"github.com/CloudyKit/jet/v6"
)

type httpFileSystemLoader struct {
	fs http.FileSystem
}

// NewLoader returns an initialized loader serving the passed http.FileSystem.
func NewLoader(fs http.FileSystem) (jet.Loader, error) {
	if fs == nil {
		return nil, errors.New("httpfs: nil http.Filesystem passed to NewLoader")
	}
	return &httpFileSystemLoader{fs: fs}, nil
}

// Open implements Loader.Open() on top of an http.FileSystem.
func (l *httpFileSystemLoader) Open(name string) (io.ReadCloser, error) {
	return l.fs.Open(name)
}

// Exists implements Loader.Exists() on top of an http.FileSystem by trying to open the file.
func (l *httpFileSystemLoader) Exists(name string) bool {
	if f, err := l.Open(name); err == nil {
		f.Close()
		return true
	}
	return false
}
