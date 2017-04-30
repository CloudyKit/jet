package utils

import (
	"reflect"
	"testing"

	"github.com/CloudyKit/jet"
)

var Set = jet.NewHTMLSet()

func TestVisitor(t *testing.T) {
	var collectedIdentifiers []string
	var mTemplate, _ = Set.LoadTemplate("_testing", "{{ ident1 }}\n{{ ident2(ident3)}}\n{{ if ident4 }}\n    {{ident5}}\n{{else}}\n    {{ident6}}\n{{end}}\n{{ ident7|ident8|ident9+ident10|ident11[ident12]: ident13[ident14:ident15] }}")
	Walk(mTemplate, VisitorFunc(func(context VisitorContext, node jet.Node) {
		if node.Type() == jet.NodeIdentifier {
			collectedIdentifiers = append(collectedIdentifiers, node.String())
		}
		context.Visit(node)
	}))
	if !reflect.DeepEqual(collectedIdentifiers, []string{"ident1", "ident2", "ident3", "ident4", "ident5", "ident6", "ident7", "ident8", "ident9", "ident10", "ident11", "ident12", "ident13", "ident14", "ident15"}) {
		t.Errorf("%q", collectedIdentifiers)
	}
}
