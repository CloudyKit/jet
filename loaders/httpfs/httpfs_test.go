package httpfs

import (
	"github.com/CloudyKit/jet"
	"github.com/CloudyKit/jet/jettest"
	"net/http"
	"testing"
)

func TestHTTPFileSystemResolve(t *testing.T) {
	fs := http.Dir("testData/includeIfNotExists")
	set := jet.NewHTMLSetLoader(NewLoader(fs))
	jettest.RunWithSet(t, set, nil, nil, "existent", "", "Hi, i exist!!")
	jettest.RunWithSet(t, set, nil, nil, "notExistent", "", "")
	jettest.RunWithSet(t, set, nil, nil, "ifIncludeIfExits", "", "Hi, i exist!!\n    Was included!!\n\n\n    Was not included!!\n\n")
	jettest.RunWithSet(t, set, nil, "World", "wcontext", "", "Hi, Buddy!\nHi, World!")
}
