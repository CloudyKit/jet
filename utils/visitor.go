package utils

import (
	"fmt"

	"github.com/CloudyKit/jet/v3"
)

// Walk walks the template ast and calls the Visit method on each node of the tree
// if you're not familiar with the Visitor pattern please check the visitor_test.go
// for usage examples
func Walk(t *jet.Template, v Visitor) {
	v.Visit(VisitorContext{Visitor: v}, t.Root)
}

// Visitor type implementing the visitor pattern
type Visitor interface {
	Visit(vc VisitorContext, node jet.Node)
}

// VisitorFunc a func that implements the Visitor interface
type VisitorFunc func(vc VisitorContext, node jet.Node)

func (visitor VisitorFunc) Visit(vc VisitorContext, node jet.Node) {
	visitor(vc, node)
}

// VisitorContext context for the current inspection
type VisitorContext struct {
	Visitor Visitor
}

func (vc VisitorContext) visitNode(node jet.Node) {
	vc.Visitor.Visit(vc, node)
}

func (vc VisitorContext) Visit(node jet.Node) {

	switch node := node.(type) {
	case *jet.ListNode:
		vc.visitListNode(node)
	case *jet.ActionNode:
		vc.visitActionNode(node)
	case *jet.ChainNode:
		vc.visitChainNode(node)
	case *jet.CommandNode:
		vc.visitCommandNode(node)
	case *jet.IfNode:
		vc.visitIfNode(node)
	case *jet.PipeNode:
		vc.visitPipeNode(node)
	case *jet.RangeNode:
		vc.visitRangeNode(node)
	case *jet.BlockNode:
		vc.visitBlockNode(node)
	case *jet.IncludeNode:
		vc.visitIncludeNode(node)
	case *jet.YieldNode:
		vc.visitYieldNode(node)
	case *jet.SetNode:
		vc.visitSetNode(node)
	case *jet.AdditiveExprNode:
		vc.visitAdditiveExprNode(node)
	case *jet.MultiplicativeExprNode:
		vc.visitMultiplicativeExprNode(node)
	case *jet.ComparativeExprNode:
		vc.visitComparativeExprNode(node)
	case *jet.NumericComparativeExprNode:
		vc.visitNumericComparativeExprNode(node)
	case *jet.LogicalExprNode:
		vc.visitLogicalExprNode(node)
	case *jet.CallExprNode:
		vc.visitCallExprNode(node)
	case *jet.NotExprNode:
		vc.visitNotExprNode(node)
	case *jet.TernaryExprNode:
		vc.visitTernaryExprNode(node)
	case *jet.IndexExprNode:
		vc.visitIndexExprNode(node)
	case *jet.SliceExprNode:
		vc.visitSliceExprNode(node)
	case *jet.TextNode:
	case *jet.IdentifierNode:
	case *jet.StringNode:
	case *jet.NilNode:
	case *jet.NumberNode:
	case *jet.BoolNode:
	case *jet.FieldNode:

	default:
		panic(fmt.Errorf("unexpected node %v", node))
	}
}

func (vc VisitorContext) visitIncludeNode(includeNode *jet.IncludeNode) {
	vc.visitNode(includeNode)
}

func (vc VisitorContext) visitBlockNode(blockNode *jet.BlockNode) {

	for _, node := range blockNode.Parameters.List {
		if node.Expression != nil {
			vc.visitNode(node.Expression)
		}
	}

	if blockNode.Expression != nil {
		vc.visitNode(blockNode.Expression)
	}

	vc.visitListNode(blockNode.List)

	if blockNode.Content != nil {
		vc.visitNode(blockNode.Content)
	}
}

func (vc VisitorContext) visitRangeNode(rangeNode *jet.RangeNode) {
	vc.visitBranchNode(&rangeNode.BranchNode)
}

func (vc VisitorContext) visitPipeNode(pipeNode *jet.PipeNode) {
	for _, node := range pipeNode.Cmds {
		vc.visitNode(node)
	}
}

func (vc VisitorContext) visitIfNode(ifNode *jet.IfNode) {
	vc.visitBranchNode(&ifNode.BranchNode)
}
func (vc VisitorContext) visitBranchNode(branchNode *jet.BranchNode) {
	if branchNode.Set != nil {
		vc.visitNode(branchNode.Set)
	}

	if branchNode.Expression != nil {
		vc.visitNode(branchNode.Expression)
	}

	vc.visitNode(branchNode.List)
	if branchNode.ElseList != nil {
		vc.visitNode(branchNode.ElseList)
	}
}

func (vc VisitorContext) visitYieldNode(yieldNode *jet.YieldNode) {
	for _, node := range yieldNode.Parameters.List {
		if node.Expression != nil {
			vc.visitNode(node.Expression)
		}
	}
	if yieldNode.Expression != nil {
		vc.visitNode(yieldNode.Expression)
	}
	if yieldNode.Content != nil {
		vc.visitNode(yieldNode.Content)
	}
}

func (vc VisitorContext) visitSetNode(setNode *jet.SetNode) {
	for _, node := range setNode.Left {
		vc.visitNode(node)
	}
	for _, node := range setNode.Right {
		vc.visitNode(node)
	}
}

func (vc VisitorContext) visitAdditiveExprNode(additiveExprNode *jet.AdditiveExprNode) {
	vc.visitNode(additiveExprNode.Left)
	vc.visitNode(additiveExprNode.Right)
}

func (vc VisitorContext) visitMultiplicativeExprNode(multiplicativeExprNode *jet.MultiplicativeExprNode) {
	vc.visitNode(multiplicativeExprNode.Left)
	vc.visitNode(multiplicativeExprNode.Right)
}

func (vc VisitorContext) visitComparativeExprNode(comparativeExprNode *jet.ComparativeExprNode) {
	vc.visitNode(comparativeExprNode.Left)
	vc.visitNode(comparativeExprNode.Right)
}

func (vc VisitorContext) visitNumericComparativeExprNode(numericComparativeExprNode *jet.NumericComparativeExprNode) {
	vc.visitNode(numericComparativeExprNode.Left)
	vc.visitNode(numericComparativeExprNode.Right)
}

func (vc VisitorContext) visitLogicalExprNode(logicalExprNode *jet.LogicalExprNode) {
	vc.visitNode(logicalExprNode.Left)
	vc.visitNode(logicalExprNode.Right)
}

func (vc VisitorContext) visitCallExprNode(callExprNode *jet.CallExprNode) {
	vc.visitNode(callExprNode.BaseExpr)
	for _, node := range callExprNode.Args {
		vc.visitNode(node)
	}
}

func (vc VisitorContext) visitNotExprNode(notExprNode *jet.NotExprNode) {
	vc.visitNode(notExprNode.Expr)
}

func (vc VisitorContext) visitTernaryExprNode(ternaryExprNode *jet.TernaryExprNode) {
	vc.visitNode(ternaryExprNode.Boolean)
	vc.visitNode(ternaryExprNode.Left)
	vc.visitNode(ternaryExprNode.Right)
}

func (vc VisitorContext) visitIndexExprNode(indexNode *jet.IndexExprNode) {
	vc.visitNode(indexNode.Base)
	vc.visitNode(indexNode.Index)
}

func (vc VisitorContext) visitSliceExprNode(sliceExprNode *jet.SliceExprNode) {
	vc.visitNode(sliceExprNode.Base)
	vc.visitNode(sliceExprNode.Index)
	vc.visitNode(sliceExprNode.EndIndex)
}

func (vc VisitorContext) visitCommandNode(commandNode *jet.CommandNode) {
	vc.visitNode(commandNode.BaseExpr)
	for _, node := range commandNode.Args {
		vc.visitNode(node)
	}
}

func (vc VisitorContext) visitChainNode(chainNode *jet.ChainNode) {
	vc.visitNode(chainNode.Node)
}

func (vc VisitorContext) visitActionNode(actionNode *jet.ActionNode) {
	if actionNode.Set != nil {
		vc.visitNode(actionNode.Set)
	}
	if actionNode.Pipe != nil {
		vc.visitNode(actionNode.Pipe)
	}
}

func (vc VisitorContext) visitListNode(listNode *jet.ListNode) {
	for _, node := range listNode.Nodes {
		vc.visitNode(node)
	}
}
