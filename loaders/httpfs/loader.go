package httpfs

import (
	"io"
	"net/http"
)

type httpFileSystemLoader struct {
	http.FileSystem
}

func JetLoader(fs http.FileSystem) httpFileSystemLoader {
	return httpFileSystemLoader{FileSystem: fs}
}

func (l httpFileSystemLoader) Open(name string) (io.ReadCloser, error) {
	f, err := l.FileSystem.Open(name)
	return f, err
}

// Exists checks if the template name exists by walking the list of template paths
// returns string with the full path of the template and bool true if the template file was found
func (l httpFileSystemLoader) Exists(name string) (string, bool) {
	f, err := l.FileSystem.Open(name)
	if err != nil {
		return "", false
	}
	f.Close()
	return name, true
}
