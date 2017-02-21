// +build !deploy_build

package templates

import (
	"net/http"
)

// Assets is not used in development and is always nil.
var Assets http.FileSystem
