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

func TestSimpleTemplate(t *testing.T) {
	var mTemplate, err = Set.LoadTemplate("_testing2", "<html><head><title>Thank you!</title></head>\n\n<body>\n\tHello {{userName}},\n\n\tThanks for the order!\n\n\t{{range product := products}}\n\t\t{{product.name}}\n\n\t    {{block productPrice(price=product.Price) product}}\n            {{if price > ExpensiveProduct}}\n                Expensive!!\n            {{end}}\n        {{end}}\n\n\t\t${{product.price / 100}}\n\t{{end}}\n</body>\n</html>")
	if err != nil {
		t.Error(err)
	}

	var (
		localVariables    []string
		externalVariables []string
	)

	Walk(mTemplate, VisitorFunc(func(context VisitorContext, node jet.Node) {
		var stackState = len(localVariables)

		switch node := node.(type) {
		case *jet.ActionNode:
			context.Visit(node)
		case *jet.SetNode:
			if node.Let {
				for _, ident := range node.Left {
					localVariables = append(localVariables, ident.String())
				}
			}
			context.Visit(node)
		case *jet.IdentifierNode:

			// skip local identifiers
			for _, varName := range localVariables {
				if varName == node.Ident {
					return
				}
			}

			// skip already inserted identifiers
			for _, varName := range externalVariables {
				if varName == node.Ident {
					return
				}
			}

			externalVariables = append(externalVariables, node.Ident)
		case *jet.BlockNode:
			for _, param := range node.Parameters.List {
				localVariables = append(localVariables, param.Identifier)
			}
			context.Visit(node)
		default:
			context.Visit(node)
			localVariables = localVariables[0:stackState]
		}

	}))

	if !reflect.DeepEqual(externalVariables, []string{"userName", "products", "ExpensiveProduct"}) {
		t.Errorf("%q", externalVariables)
	}
}
