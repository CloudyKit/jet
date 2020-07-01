// Copyright 2016 José Santos <henrique_1609@me.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jet

import (
	"io"
	"os"
	"path/filepath"
)

// Loader is a minimal interface required for loading templates.
type Loader interface {
	// Exists checks for template existence.
	Exists(path string) (string, bool)
	// Open opens the underlying reader with template content.
	Open(path string) (io.ReadCloser, error)
}

// OSFileSystemLoader implements Loader interface using OS file system (os.File).
type OSFileSystemLoader struct {
	dir string
}

// compile time check that we implement Loader
var _ Loader = (*OSFileSystemLoader)(nil)

// NewOSFileSystemLoader returns an initialized OSFileSystemLoader.
func NewOSFileSystemLoader(dirPath string) *OSFileSystemLoader {
	return &OSFileSystemLoader{
		dir: dirPath,
	}
}

// Open opens a file from OS file system.
func (l *OSFileSystemLoader) Open(path string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.dir, path))
}

// Exists checks if the template name exists by walking the list of template paths
// returns true if the template file was found
func (l *OSFileSystemLoader) Exists(path string) (string, bool) {
	path = filepath.Join(l.dir, path)
	stat, err := os.Stat(path)
	if err == nil && !stat.IsDir() {
		return path, true
	}
	return "", false
}
