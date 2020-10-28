package utils

import (
	"reflect"
	"testing"

	"github.com/CloudyKit/jet/v5"
)

var (
	Loader = jet.NewInMemLoader()
	Set    = jet.NewSet(Loader)
)

func TestVisitor(t *testing.T) {
	var collectedIdentifiers []string
	Loader.Set("_testing", "{{ ident1 }}\n{{ ident2(ident3)}}\n{{ if ident4 }}\n    {{ident5}}\n{{else}}\n    {{ident6}}\n{{end}}\n{{ ident7|ident8|ident9+ident10|ident11[ident12]: ident13[ident14:ident15] }}")
	mTemplate, _ := Set.GetTemplate("_testing")
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
	Loader.Set("_testing2", "<html><head><title>Thank you!</title></head>\n\n<body>\n\tHello {{userName}},\n\n\tThanks for the order!\n\n\t{{range product := products}}\n\t\t{{product.name}}\n\n\t    {{block productPrice(price=product.Price) product}}\n            {{if price > ExpensiveProduct}}\n                Expensive!!\n            {{end}}\n        {{end}}\n\n\t\t${{product.price / 100}}\n\t{{end}}\n</body>\n</html>")
	mTemplate, err := Set.GetTemplate("_testing2")
	if err != nil {
		t.Error(err)
	}

	var (
		localVariables    []string
		externalVariables []string
	)

	Walk(mTemplate, VisitorFunc(func(context VisitorContext, node jet.Node) {

		var stackState = len(localVariables) // saves the state of the local identifiers map

		switch node := node.(type) {
		case *jet.SetNode:
			if node.Let { // check if this is setting a new variable in the current scope
				for _, ident := range node.Left {
					// push local identifier
					localVariables = append(localVariables, ident.String())
				}
			}
			// continue checking nodes down the tree
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

			// push external identifier
			externalVariables = append(externalVariables, node.Ident)
		case *jet.ActionNode:
			// continue without restore state of local identifiers map
			context.Visit(node)
		case *jet.BlockNode:

			// iterate over block parameters
			for _, param := range node.Parameters.List {
				// store block parameters in the local map
				localVariables = append(localVariables, param.Identifier)
			}

			// continue down tree
			context.Visit(node)
			// restore local identifiers map
			localVariables = localVariables[0:stackState]
		default:
			// continue down tree
			context.Visit(node)
			// restore local identifiers map
			localVariables = localVariables[0:stackState]
		}

	}))

	if !reflect.DeepEqual(externalVariables, []string{"userName", "products", "ExpensiveProduct"}) {
		t.Errorf("%q", externalVariables)
	}
}
