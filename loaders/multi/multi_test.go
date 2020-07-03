package multi

import (
	"net/http"
	"testing"

	"github.com/CloudyKit/jet/v4"
	"github.com/CloudyKit/jet/v4/jettest"
	"github.com/CloudyKit/jet/v4/loaders/httpfs"
)

func TestZeroLoaders(t *testing.T) {
	const fileName = "does-not-exists.jet"
	l := NewLoader()
	if _, err := l.Open("does-not-exist.jet"); err == nil {
		t.Fatal("Open should have returned an error but didn't.")
	}
	fullPath, ok := l.Exists(fileName)
	if ok {
		t.Fatalf("Exists called on an empty file system should have returned empty and false but reported the template exists under the full path %q", fullPath)
	}
}

func TestTwoLoaders(t *testing.T) {
	osFSLoader := jet.NewOSFileSystemLoader("./testData")
	httpFSLoader := httpfs.NewLoader(http.Dir("../../testData"))
	l := NewLoader(osFSLoader, httpFSLoader)
	set := jet.NewHTMLSetLoader(l)
	jettest.RunWithSet(t, set, nil, nil, "resolve/simple.jet", "", "simple.jet")
	jettest.RunWithSet(t, set, nil, nil, "base.jet", "", "")
	jettest.RunWithSet(t, set, nil, nil, "simple2", "", "simple2\n")
}
