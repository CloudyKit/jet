package embedfs

import (
	"embed"
	"testing"

	"github.com/CloudyKit/jet/v6"
	"github.com/CloudyKit/jet/v6/jettest"
)

//go:embed testData/includeIfNotExists/*
var templateFS embed.FS

func TestEmbedFileSystemResolve(t *testing.T) {
	l := NewLoader("testData/includeIfNotExists", templateFS)

	set := jet.NewSet(l)
	jettest.RunWithSet(t, set, nil, nil, "existent", "Hi, i exist!!")
	jettest.RunWithSet(t, set, nil, nil, "notExistent", "")
	jettest.RunWithSet(t, set, nil, nil, "ifIncludeIfExits", "Hi, i exist!!\n    Was included!!\n\n\n    Was not included!!\n\n")
	jettest.RunWithSet(t, set, nil, "World", "wcontext", "Hi, Buddy!\nHi, World!")
}
