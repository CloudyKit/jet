package multi

import (
	"io"
	"os"

	"github.com/CloudyKit/jet/v3"
)

// Multi implements jet.Loader interface and tries to load templates from a list of custom loaders.
type Multi struct {
	loaders []jet.Loader
}

// NewLoader returns a new multi loader. The order of the loaders passed as parameters
// will define the order in which templates are loaded.
func NewLoader(loaders ...jet.Loader) *Multi {
	return &Multi{loaders: loaders}
}

// AddLoaders adds the passed loaders to the list of loaders.
func (m *Multi) AddLoaders(loaders ...jet.Loader) {
	m.loaders = append(m.loaders, loaders...)
}

// ClearLoaders clears the list of loaders.
func (m *Multi) ClearLoaders() {
	m.loaders = nil
}

// Open will open the file passed by trying all loaders in succession.
func (m *Multi) Open(name string) (io.ReadCloser, error) {
	for _, loader := range m.loaders {
		if f, err := loader.Open(name); err == nil {
			return f, nil
		}
	}
	return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
}

// Exists checks all loaders in succession, returning the full path of the template and
// bool true if the template file was found, otherwise it returns an empty string and false.
func (m *Multi) Exists(name string) (string, bool) {
	for _, loader := range m.loaders {
		if fileName, ok := loader.Exists(name); ok {
			return fileName, true
		}
	}
	return "", false
}
