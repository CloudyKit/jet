package multi

import (
	"net/http"
	"testing"

	"github.com/CloudyKit/jet/v3"
	"github.com/CloudyKit/jet/v3/jettest"
	"github.com/CloudyKit/jet/v3/loaders/httpfs"
)

func TestZeroLoaders(t *testing.T) {
	l := NewLoader()
	if _, err := l.Open("does-not-exist.jet"); err == nil {
		t.Fatal("Open should have returned an error but didn't.")
	}
	fileName, ok := l.Exists("does-not-exists.jet")
	if fileName != "" || ok != false {
		t.Fatalf("Exists called on an empty file system should have returned empty and false but returned %q and %+v", fileName, ok)
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
