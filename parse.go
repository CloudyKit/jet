// Copyright 2016 José Santos <henrique_1609@me.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package jet

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func unquote(text string) (string, error) {
	if text[0] == '@' {
		return text[1:], nil
	}
	return strconv.Unquote(text)
}

// parser is the representation of a single parsed template.
type Template struct {
	Name      string // name of the template represented by the tree.
	ParseName string // name of the top-level template during parsing, for error messages.
	set       *Set

	extends *Template
	imports []*Template

	processedBlocks map[string]*BlockNode
	passedBlocks    map[string]*BlockNode
	root            *ListNode // top-level root of the tree.

	text string // text parsed to create the template (or its parent)

	// Parsing only; cleared after parse.
	lex       *lexer
	token     [3]item // three-token lookahead for parser.
	peekCount int
}

// next returns the next token.
func (t *Template) next() item {
	if t.peekCount > 0 {
		t.peekCount--
	} else {
		t.token[0] = t.lex.nextItem()
	}
	return t.token[t.peekCount]
}

// backup backs the input stream up one token.
func (t *Template) backup() {
	t.peekCount++
}

// backup2 backs the input stream up two tokens.
// The zeroth token is already there.
func (t *Template) backup2(t1 item) {
	t.token[1] = t1
	t.peekCount = 2
}

// backup3 backs the input stream up three tokens
// The zeroth token is already there.
func (t *Template) backup3(t2, t1 item) {
	// Reverse order: we're pushing back.
	t.token[1] = t1
	t.token[2] = t2
	t.peekCount = 3
}

// peek returns but does not consume the next token.
func (t *Template) peek() item {
	if t.peekCount > 0 {
		return t.token[t.peekCount-1]
	}
	t.peekCount = 1
	t.token[0] = t.lex.nextItem()
	return t.token[0]
}

// nextNonSpace returns the next non-space token.
func (t *Template) nextNonSpace() (token item) {
	for {
		token = t.next()
		if token.typ != itemSpace {
			break
		}
	}
	return token
}

// peekNonSpace returns but does not consume the next non-space token.
func (t *Template) peekNonSpace() (token item) {
	for {
		token = t.next()
		if token.typ != itemSpace {
			break
		}
	}
	t.backup()
	return token
}

// errorf formats the error and terminates processing.
func (t *Template) errorf(format string, args ...interface{}) {
	t.root = nil
	format = fmt.Sprintf("template: %s:%d: %s", t.ParseName, t.lex.lineNumber(), format)
	panic(fmt.Errorf(format, args...))
}

// error terminates processing.
func (t *Template) error(err error) {
	t.errorf("%s", err)
}

// expect consumes the next token and guarantees it has the required type.
func (t *Template) expect(expected itemType, context string) item {
	token := t.nextNonSpace()
	if token.typ != expected {
		t.unexpected(token, context)
	}
	return token
}

// expectOneOf consumes the next token and guarantees it has one of the required types.
func (t *Template) expectOneOf(expected1, expected2 itemType, context string) item {
	token := t.nextNonSpace()
	if token.typ != expected1 && token.typ != expected2 {
		t.unexpected(token, context)
	}
	return token
}

// unexpected complains about the token and terminates processing.
func (t *Template) unexpected(token item, context string) {
	t.errorf("unexpected %s in %s", token, context)
}

// recover is the handler that turns panics into returns from the top level of Parse.
func (t *Template) recover(errp *error) {
	e := recover()
	if e != nil {
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		if t != nil {
			t.lex.drain()
			t.stopParse()
		}
		*errp = e.(error)
	}
	return
}

func (s *Set) parse(name, text string) (t *Template, err error) {
	t = &Template{Name: name, text: text, set: s, passedBlocks: make(map[string]*BlockNode)}
	defer t.recover(&err)

	t.ParseName = t.Name
	t.startParse(lex(t.Name, text))
	t.parseTemplate()
	t.stopParse()

	if t.extends != nil {
		t.addBlocks(t.extends.processedBlocks)
	}
	for _, _import := range t.imports {
		t.addBlocks(_import.processedBlocks)
	}
	t.addBlocks(t.passedBlocks)
	return t, nil
}

func (t *Template) expectString(context string) string {
	token := t.expectOneOf(itemString, itemRawString, context)
	s, err := unquote(token.val)
	if err != nil {
		t.error(err)
	}
	return s
}

// parse is the top-level parser for a template, essentially the same
// It runs to EOF.
func (t *Template) parseTemplate() (next Node) {
	t.root = t.newList(t.peek().pos)
	// {{ extends|import stringLiteral }}
	for t.peek().typ != itemEOF {
		delim := t.next()
		if delim.typ == itemText && strings.TrimSpace(delim.val) == "" {
			continue //skips empty text nodes
		}
		if delim.typ == itemLeftDelim {
			token := t.nextNonSpace()
			if token.typ == itemExtends || token.typ == itemImport {
				s := t.expectString("extends|import")
				if token.typ == itemExtends {
					if t.extends != nil {
						t.errorf("Unexpected extends clause, only one extends clause is valid per template")
					} else if len(t.imports) > 0 {
						t.errorf("Unexpected extends clause, all import clause should come after extends clause")
					}
					var err error
					t.extends, err = t.set.loadTemplate(s, "")
					if err != nil {
						t.error(err)
					}
				} else {
					tt, err := t.set.loadTemplate(s, "")
					if err != nil {
						t.error(err)
					}
					t.imports = append(t.imports, tt)
				}
				t.expect(itemRightDelim, "extends|import")
			} else {
				t.backup2(delim)
				break
			}
		} else {
			t.backup()
			break
		}
	}

	for t.peek().typ != itemEOF {
		switch n := t.textOrAction(); n.Type() {
		case nodeEnd, nodeElse:
			t.errorf("unexpected %s", n)
		default:
			t.root.append(n)
		}
	}
	return nil
}

// startParse initializes the parser, using the lexer.
func (t *Template) startParse(lex *lexer) {
	t.root = nil
	t.lex = lex
}

// stopParse terminates parsing.
func (t *Template) stopParse() {
	t.lex = nil
}

// IsEmptyTree reports whether this tree (node) is empty of everything but space.
func IsEmptyTree(n Node) bool {
	switch n := n.(type) {
	case nil:
		return true
	case *ActionNode:
	case *IfNode:
	case *ListNode:
		for _, node := range n.Nodes {
			if !IsEmptyTree(node) {
				return false
			}
		}
		return true
	case *RangeNode:
	case *IncludeNode:
	case *TextNode:
		return len(bytes.TrimSpace(n.Text)) == 0
	case *BlockNode:
	case *YieldNode:
	default:
		panic("unknown node: " + n.String())
	}
	return false
}

// parseDefinition parses a {{block Ident pipeline?}} ...  {{end}} template definition and
// installs the definition in the treeSet map.  The "define" keyword has already
// been scanned.
func (t *Template) parseBlock() Node {
	const context = "block clause"
	name := t.expect(itemIdentifier, context)

	var pipe Expression

	if t.peekNonSpace().typ != itemRightDelim {
		pipe = t.expression("block")
	}
	t.expect(itemRightDelim, context)
	list, end := t.itemList()
	if end.Type() != nodeEnd {
		t.errorf("unexpected %s in %s", end, context)
	}

	block := t.newBlock(name.pos, t.lex.lineNumber(), name.val, pipe, list)
	t.passedBlocks[block.Name] = block
	return block
}

func (t *Template) parseYield() Node {
	const context = "yield clause"
	var pipe Expression

	name := t.expect(itemIdentifier, context)

	if t.peekNonSpace().typ != itemRightDelim {
		pipe = t.expression("yield")
	}
	t.expect(itemRightDelim, context)
	return t.newYield(name.pos, t.lex.lineNumber(), name.val, pipe)
}

func (t *Template) parseInclude() Node {
	var name string
	token := t.nextNonSpace()

	switch token.typ {
	case itemString, itemRawString:
		s, err := unquote(token.val)
		if err != nil {
			t.error(err)
		}
		name = s
	default:
		t.unexpected(token, "include invocation")
	}

	var pipe Expression
	if t.nextNonSpace().typ != itemRightDelim {
		t.backup()
		pipe = t.expression("include")
	}

	return t.newInclude(token.pos, t.lex.lineNumber(), name, pipe)
}

// itemList:
//	textOrAction*
// Terminates at {{end}} or {{else}}, returned separately.
func (t *Template) itemList() (list *ListNode, next Node) {
	list = t.newList(t.peekNonSpace().pos)
	for t.peekNonSpace().typ != itemEOF {
		n := t.textOrAction()
		switch n.Type() {
		case nodeEnd, nodeElse:
			return list, n
		}
		list.append(n)
	}
	t.errorf("unexpected EOF")
	return
}

// textOrAction:
//	text | action
func (t *Template) textOrAction() Node {
	switch token := t.nextNonSpace(); token.typ {
	case itemText:
		return t.newText(token.pos, token.val)
	case itemLeftDelim:
		return t.action()
	default:
		t.unexpected(token, "input")
	}
	return nil
}

// Action:
//	control
//	command ("|" command)*
// Left delim is past. Now get actions.
// First word could be a keyword such as range.
func (t *Template) action() (n Node) {
	switch token := t.nextNonSpace(); token.typ {
	case itemElse:
		return t.elseControl()
	case itemEnd:
		return t.endControl()
	case itemIf:
		return t.ifControl()
	case itemRange:
		return t.rangeControl()
	case itemBlock:
		return t.parseBlock()
	case itemInclude:
		return t.parseInclude()
	case itemYield:
		return t.parseYield()

	}
	t.backup()

	action := t.newAction(t.peek().pos, t.lex.lineNumber())

	if t.peekNonSpace().typ == itemSet {
		t.next()
		action.Set = t.assignmentOrExpression("command").(*SetNode)
		t.peekNonSpace()
	}

	if action.Set == nil || t.expectOneOf(itemColonComma, itemRightDelim, "command").typ == itemColonComma {
		action.Pipe = t.pipeline("command")
	}
	return action
}

func (t *Template) logicalExpression(context string) (Expression, item) {
	left, endtoken := t.comparativeExpression(context)
	for endtoken.typ == itemAnd || endtoken.typ == itemOr {
		right, rightendtoken := t.comparativeExpression(context)
		left, endtoken = t.newLogicalExpr(left.Position(), t.lex.lineNumber(), left, right, endtoken), rightendtoken
	}
	return left, endtoken
}

func (t *Template) comparativeExpression(context string) (Expression, item) {
	left, endtoken := t.numericComparativeExpression(context)
	for endtoken.typ == itemEquals || endtoken.typ == itemNotEquals {
		right, rightendtoken := t.numericComparativeExpression(context)
		left, endtoken = t.newComparativeExpr(left.Position(), t.lex.lineNumber(), left, right, endtoken), rightendtoken
	}
	return left, endtoken
}

func (t *Template) numericComparativeExpression(context string) (Expression, item) {
	left, endtoken := t.additiveExpression(context)
	for endtoken.typ >= itemGreat && endtoken.typ <= itemLessEquals {
		right, rightendtoken := t.additiveExpression(context)
		left, endtoken = t.newNumericComparativeExpr(left.Position(), t.lex.lineNumber(), left, right, endtoken), rightendtoken
	}
	return left, endtoken
}

func (t *Template) additiveExpression(context string) (Expression, item) {
	left, endtoken := t.multiplicativeExpression(context)
	for endtoken.typ == itemAdd || endtoken.typ == itemMinus {
		right, rightendtoken := t.multiplicativeExpression(context)
		left, endtoken = t.newAdditiveExpr(left.Position(), t.lex.lineNumber(), left, right, endtoken), rightendtoken
	}
	return left, endtoken
}

func (t *Template) multiplicativeExpression(context string) (left Expression, endtoken item) {
	left, endtoken = t.unaryExpression(context)
	for endtoken.typ >= itemMul && endtoken.typ <= itemMod {
		right, rightendtoken := t.unaryExpression(context)
		left, endtoken = t.newMultiplicativeExpr(left.Position(), t.lex.lineNumber(), left, right, endtoken), rightendtoken
	}

	return left, endtoken
}

func (t *Template) unaryExpression(context string) (Expression, item) {
	next := t.nextNonSpace()
	if next.typ == itemNot {
		expr, endToken := t.comparativeExpression(context)
		return t.newNotExpr(expr.Position(), t.lex.lineNumber(), expr), endToken
	}
	t.backup()
	operand := t.operand()
	return operand, t.nextNonSpace()
}

func (t *Template) assignmentOrExpression(context string) (operand Expression) {

	t.peekNonSpace()
	pos := t.lex.pos
	line := t.lex.lineNumber()
	var right, left []Expression

	var isSet bool
	var isLet bool
	var returned item
	operand, returned = t.logicalExpression(context)
	if returned.typ == itemComma || returned.typ == itemAssign {
		isSet = true
	} else {
		if operand == nil {
			t.unexpected(returned, context)
		}
		t.backup()
		return operand
	}

	if isSet {
	leftloop:
		for {
			switch operand.Type() {
			case NodeField, NodeChain, NodeIdentifier:
				left = append(left, operand)
			default:
				t.errorf("unexpected node in assign")
			}

			switch returned.typ {
			case itemComma:
				operand, returned = t.logicalExpression(context)
			case itemAssign:
				isLet = returned.val == ":="
				break leftloop
			default:
				t.unexpected(returned, "assignment")
			}
		}

		if isLet {
			for _, operand := range left {
				if operand.Type() != NodeIdentifier {
					t.errorf("unexpected node type %s in variable declaration", operand)
				}
			}
		}

		for {
			operand, returned = t.logicalExpression("assignment")
			right = append(right, operand)
			if returned.typ != itemComma {
				t.backup()
				break
			}
		}

		if context == "range" {
			if len(left) > 2 || len(right) > 1 {
				t.errorf("unexpected number of operands in assign on range")
			}
		} else {
			if len(left) != len(right) {
				t.errorf("unexpected number of operands in assign on range")
			}
		}
		operand = t.newSet(pos, line, isLet, left, right)
		return

	}
	return
}

func (t *Template) expression(context string) Expression {
	pipe, tk := t.logicalExpression(context)
	if pipe == nil {
		t.unexpected(tk, context)
	}
	t.backup()
	return pipe
}

// Pipeline:
//	declarations? command ('|' command)*
func (t *Template) pipeline(context string) (pipe *PipeNode) {
	pos := t.peekNonSpace().pos
	pipe = t.newPipeline(pos, t.lex.lineNumber())
	token := t.nextNonSpace()
loop:
	for {
		switch token.typ {
		case itemBool, itemCharConstant, itemComplex, itemField, itemIdentifier,
			itemNumber, itemNil, itemRawString, itemString, itemLeftParen, itemNot:
			t.backup()
			pipe.append(t.command())
			token = t.nextNonSpace()
			if token.typ == itemPipe {
				token = t.nextNonSpace()
				continue loop
			} else {
				t.backup()
				break loop
			}
		default:
			t.backup()
			break loop
		}
	}

	t.expect(itemRightDelim, context)
	return
}

// command:
//	operand (:(space operand)*)?
// space-separated arguments up to a pipeline character or right delimiter.
// we consume the pipe character but leave the right delim to terminate the action.
func (t *Template) command() *CommandNode {
	cmd := t.newCommand(t.peekNonSpace().pos)

	cmd.BaseExpr = t.expression("command")
	if t.nextNonSpace().typ == itemColon {
		cmd.Call = true
		cmd.Args = t.parseArguments()
	} else {
		t.backup()
	}

	if cmd.BaseExpr == nil {
		t.errorf("empty command")
	}
	return cmd
}

// operand:
//	term .Field*
// An operand is a space-separated component of a command,
// a term possibly followed by field accesses.
// A nil return means the next item is not an operand.
func (t *Template) operand() Expression {
	node := t.term()
	if node == nil {
		t.errorf("unexpected token %s on operand", t.next())
	}
RESET:
	if t.peek().typ == itemField {
		chain := t.newChain(t.peek().pos, node)
		for t.peek().typ == itemField {
			chain.Add(t.next().val)
		}
		// Compatibility with original API: If the term is of type NodeField
		// or NodeVariable, just put more fields on the original.
		// Otherwise, keep the Chain node.
		// Obvious parsing errors involving literal values are detected here.
		// More complex error cases will have to be handled at execution time.
		switch node.Type() {
		case NodeField:
			node = t.newField(chain.Position(), chain.String())
		case NodeBool, NodeString, NodeNumber, NodeNil:
			t.errorf("unexpected . after term %q", node.String())
		default:
			node = chain
		}

	}
	if t.nextNonSpace().typ == itemLeftParen {
		callExpr := t.newCallExpr(node.Position(), t.lex.lineNumber(), node)
		callExpr.Args = t.parseArguments()
		t.expect(itemRightParen, "call expression")
		node = callExpr
		goto RESET
	} else {
		t.backup()
	}
	return node
}

func (t *Template) parseArguments() (args []Expression) {
	if t.peekNonSpace().typ != itemRightParen {
	loop:
		for {
			expr, endtoken := t.logicalExpression("call expression")
			args = append(args, expr)
			switch endtoken.typ {
			case itemComma:
				continue loop
			default:
				t.backup()
				break loop
			}
		}
	}
	return
}

func (t *Template) checkPipeline(pipe *PipeNode, context string) {

	// GetProductById productId -> Field Name -> html
	// GetProductById productId -> Method GetCategories -> Select

	// Reject empty pipelines
	if len(pipe.Cmds) == 0 {
		t.errorf("missing value for %s", context)
	}

	// Only the first command of a pipeline can start with a non executable operand
	for i, c := range pipe.Cmds[1:] {
		switch c.Args[0].Type() {
		case NodeBool, NodeNil, NodeNumber, NodeString:
			// With A|B|C, pipeline stage 2 is B
			t.errorf("non executable command in pipeline stage %d", i+2)
		}
	}
}

func (t *Template) parseControl(allowElseIf bool, context string) (pos Pos, line int, set *SetNode, expression Expression, list, elseList *ListNode) {
	line = t.lex.lineNumber()
	pos = t.lex.pos

	expression = t.assignmentOrExpression(context)
	//if expression == nil {
	//	println("nil here",t.lex.input[0:t.lex.pos])
	//}
	if expression.Type() == NodeSet {
		set = expression.(*SetNode)
		if context != "range" {
			t.expect(itemColonComma, context)
			expression = t.expression(context)
		} else {
			expression = nil
		}
	}

	t.expect(itemRightDelim, context)
	var next Node
	list, next = t.itemList()
	switch next.Type() {
	case nodeEnd: //done
	case nodeElse:
		if allowElseIf {
			// Special case for "else if". If the "else" is followed immediately by an "if",
			// the elseControl will have left the "if" token pending. Treat
			//	{{if a}}_{{else if b}}_{{end}}
			// as
			//	{{if a}}_{{else}}{{if b}}_{{end}}{{end}}.
			// To do this, parse the if as usual and stop at it {{end}}; the subsequent{{end}}
			// is assumed. This technique works even for long if-else-if chains.
			// TODO: Should we allow else-if in with and range?
			if t.peek().typ == itemIf {
				t.next() // Consume the "if" token.
				elseList = t.newList(next.Position())
				elseList.append(t.ifControl())
				// Do not consume the next item - only one {{end}} required.
				break
			}
		}
		elseList, next = t.itemList()
		if next.Type() != nodeEnd {
			t.errorf("expected end; found %s", next)
		}
	}
	return pos, line, set, expression, list, elseList
}

// If:
//	{{if pipeline}} itemList {{end}}
//	{{if pipeline}} itemList {{else}} itemList {{end}}
// If keyword is past.
func (t *Template) ifControl() Node {
	return t.newIf(t.parseControl(true, "if"))
}

// Range:
//	{{range pipeline}} itemList {{end}}
//	{{range pipeline}} itemList {{else}} itemList {{end}}
// Range keyword is past.
func (t *Template) rangeControl() Node {
	return t.newRange(t.parseControl(false, "range"))
}

// End:
//	{{end}}
// End keyword is past.
func (t *Template) endControl() Node {
	return t.newEnd(t.expect(itemRightDelim, "end").pos)
}

// Else:
//	{{else}}
// Else keyword is past.
func (t *Template) elseControl() Node {
	// Special case for "else if".
	peek := t.peekNonSpace()
	if peek.typ == itemIf {
		// We see "{{else if ... " but in effect rewrite it to {{else}}{{if ... ".
		return t.newElse(peek.pos, t.lex.lineNumber())
	}
	return t.newElse(t.expect(itemRightDelim, "else").pos, t.lex.lineNumber())
}

// term:
//	literal (number, string, nil, boolean)
//	function (identifier)
//	.
//	.Field
//	$
//	'(' pipeline ')'
// A term is a simple "expression".
// A nil return means the next item is not a term.
func (t *Template) term() Node {
	switch token := t.nextNonSpace(); token.typ {
	case itemError:
		t.errorf("%s", token.val)
	case itemIdentifier:
		return t.newIdentifier(token.val, token.pos, t.lex.lineNumber())
	case itemNil:
		return t.newNil(token.pos)
	case itemField:
		return t.newField(token.pos, token.val)
	case itemBool:
		return t.newBool(token.pos, token.val == "true")
	case itemCharConstant, itemComplex, itemNumber:
		number, err := t.newNumber(token.pos, token.val, token.typ)
		if err != nil {
			t.error(err)
		}
		return number
	case itemLeftParen:
		pipe := t.expression("parenthesized expression")
		if token := t.next(); token.typ != itemRightParen {
			t.errorf("unclosed right paren: unexpected %s", token)
		}
		return pipe
	case itemString, itemRawString:
		s, err := unquote(token.val)
		if err != nil {
			t.error(err)
		}
		return t.newString(token.pos, token.val, s)
	}
	t.backup()
	return nil
}
