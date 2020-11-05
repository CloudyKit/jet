package multi

import (
	"net/http"
	"testing"

	"github.com/CloudyKit/jet/v6"
	"github.com/CloudyKit/jet/v6/jettest"
	"github.com/CloudyKit/jet/v6/loaders/httpfs"
)

func TestZeroLoaders(t *testing.T) {
	const fileName = "does-not-exists.jet"
	l := NewLoader()
	if _, err := l.Open("does-not-exist.jet"); err == nil {
		t.Fatal("Open should have returned an error but didn't.")
	}
	ok := l.Exists(fileName)
	if ok {
		t.Fatal("Exists called on an empty file system should have returned empty and false but reported true")
	}
}

func TestTwoLoaders(t *testing.T) {
	osFSLoader := jet.NewOSFileSystemLoader("./testData")
	httpFSLoader, err := httpfs.NewLoader(http.Dir("../../testData"))
	if err != nil {
		t.Fatalf("unexpected error from httpfs.NewLoader: %v", err)
	}
	l := NewLoader(osFSLoader, httpFSLoader)
	set := jet.NewSet(l)
	jettest.RunWithSet(t, set, nil, nil, "resolve/simple.jet", "simple.jet")
	jettest.RunWithSet(t, set, nil, nil, "base.jet", "")
	jettest.RunWithSet(t, set, nil, nil, "simple2", "simple2\n")
}
