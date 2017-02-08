package jet

import "io"

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
