package embedfs

import (
	"embed"
	"io"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/CloudyKit/jet/v6"
)

type embedFileSystemLoader struct {
	dir string
	fs  embed.FS
}

// NewLoader returns an initialized loader serving the passed embed.FS.
func NewLoader(dirPath string, fs embed.FS) jet.Loader {
	return &embedFileSystemLoader{
		dir: path.Clean(filepath.ToSlash(dirPath)),
		fs:  fs,
	}
}

// Open implements Loader.Open() on top of an embed.FS.
func (l *embedFileSystemLoader) Open(name string) (io.ReadCloser, error) {
	return l.fs.Open(path.Join(l.dir, path.Clean(filepath.ToSlash(name))))
}

// Exists implements Loader.Exists() on top of an embed.FS by trying to open the file.
func (l *embedFileSystemLoader) Exists(name string) bool {
	name = path.Join(l.dir, path.Clean(filepath.ToSlash(name)))
	stat, err := fs.Stat(l.fs, name)
	if err == nil && !stat.IsDir() {
		return true
	}
	return false
}
