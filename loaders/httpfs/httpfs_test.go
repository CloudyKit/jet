package httpfs

import (
	"net/http"
	"testing"

	"github.com/CloudyKit/jet/v6"
	"github.com/CloudyKit/jet/v6/jettest"
)

func TestNilHTTPFileSystem(t *testing.T) {
	const fileName = "does-not-exists.jet"
	_, err := NewLoader(nil)
	if err == nil {
		t.Fatal("NewLoader with nil http.FileSystem should have returned an error but didn't.")
	}
}

func TestHTTPFileSystemResolve(t *testing.T) {
	l, err := NewLoader(http.Dir("testData/includeIfNotExists"))
	if err != nil {
		t.Fatalf("unexpected error from NewLoader: %v", err)
	}
	set := jet.NewSet(l)
	jettest.RunWithSet(t, set, nil, nil, "existent", "Hi, i exist!!")
	jettest.RunWithSet(t, set, nil, nil, "notExistent", "")
	jettest.RunWithSet(t, set, nil, nil, "ifIncludeIfExits", "Hi, i exist!!\n    Was included!!\n\n\n    Was not included!!\n\n")
	jettest.RunWithSet(t, set, nil, "World", "wcontext", "Hi, Buddy!\nHi, World!")
}
