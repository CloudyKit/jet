package httpfs

import (
	"net/http"
	"testing"

	"github.com/CloudyKit/jet/v4"
	"github.com/CloudyKit/jet/v4/jettest"
)

func TestNilHTTPFileSystem(t *testing.T) {
	const fileName = "does-not-exists.jet"
	l := NewLoader(nil)
	if _, err := l.Open(fileName); err == nil {
		t.Fatal("Open should have returned an error but didn't.")
	}
	actualName, ok := l.Exists(fileName)
	if ok {
		t.Fatalf("Exists called on an empty file system should have returned empty and false but said the template exists under the name %q", actualName)
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
