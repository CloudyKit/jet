package httpfs

import (
	"net/http"
	"testing"

	"github.com/CloudyKit/jet/v3"
	"github.com/CloudyKit/jet/v3/jettest"
)

func TestNilHTTPFileSystem(t *testing.T) {
	l := NewLoader(nil)
	if _, err := l.Open("does-not-exist.jet"); err == nil {
		t.Fatal("Open should have returned an error but didn't.")
	}
	fileName, ok := l.Exists("does-not-exists.jet")
	if fileName != "" || ok != false {
		t.Fatalf("Exists called on an empty file system should have returned empty and false but returned %q and %+v", fileName, ok)
	}
}

func TestHTTPFileSystemResolve(t *testing.T) {
	fs := http.Dir("testData/includeIfNotExists")
	set := jet.NewHTMLSetLoader(NewLoader(fs))
	jettest.RunWithSet(t, set, nil, nil, "existent", "", "Hi, i exist!!")
	jettest.RunWithSet(t, set, nil, nil, "notExistent", "", "")
	jettest.RunWithSet(t, set, nil, nil, "ifIncludeIfExits", "", "Hi, i exist!!\n    Was included!!\n\n\n    Was not included!!\n\n")
	jettest.RunWithSet(t, set, nil, "World", "wcontext", "", "Hi, Buddy!\nHi, World!")
}
