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
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/CloudyKit/fastprinter"
)

var (
	funcType       = reflect.TypeOf(Func(nil))
	stringerType   = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
	rangerType     = reflect.TypeOf((*Ranger)(nil)).Elem()
	rendererType   = reflect.TypeOf((*Renderer)(nil)).Elem()
	safeWriterType = reflect.TypeOf(SafeWriter(nil))
	pool_State     = sync.Pool{
		New: func() interface{} {
			return &Runtime{scope: &scope{}, escapeeWriter: new(escapeeWriter)}
		},
	}
)

// Renderer any resulting value from an expression in an action that implements this
// interface will not be printed, instead, we will invoke his Render() method which will be responsible
// to render his self
type Renderer interface {
	Render(*Runtime)
}

// RendererFunc func implementing interface Renderer
type RendererFunc func(*Runtime)

func (renderer RendererFunc) Render(r *Runtime) {
	renderer(r)
}

// Ranger a value implementing a ranger interface is able to iterate on his value
// and can be used directly in a range statement
type Ranger interface {
	Range() (reflect.Value, reflect.Value, bool)
}

type escapeeWriter struct {
	Writer  io.Writer
	escapee SafeWriter
	set     *Set
}

func (w *escapeeWriter) Write(b []byte) (int, error) {
	if w.set.escapee == nil {
		w.Writer.Write(b)
	} else {
		w.set.escapee(w.Writer, b)
	}
	return 0, nil
}

// Runtime this type holds the state of the execution of an template
type Runtime struct {
	*escapeeWriter
	*scope
	content func(*Runtime, Expression)

	translator Translator
	context    reflect.Value
}

// Context returns the current context value
func (r *Runtime) Context() reflect.Value {
	return r.context
}

func (st *Runtime) newScope() {
	st.scope = &scope{parent: st.scope, variables: make(VarMap), blocks: st.blocks}
}

func (st *Runtime) releaseScope() {
	st.scope = st.scope.parent
}

type scope struct {
	parent    *scope
	variables VarMap
	blocks    map[string]*BlockNode
}

// YieldBlock yields a block in the current context, will panic if the context is not available
func (st *Runtime) YieldBlock(name string, context interface{}) {
	block, has := st.getBlock(name)

	if has == false {
		panic(fmt.Errorf("Block %q was not found!!", name))
	}

	if context != nil {
		current := st.context
		st.context = reflect.ValueOf(context)
		st.executeList(block.List)
		st.context = current
	}

	st.executeList(block.List)
}

func (st *scope) getBlock(name string) (block *BlockNode, has bool) {
	block, has = st.blocks[name]
	for !has && st.parent != nil {
		st = st.parent
		block, has = st.blocks[name]
	}
	return
}

// YieldTemplate yields a template same as include
func (st *Runtime) YieldTemplate(name string, context interface{}) {

	t, err := st.set.GetTemplate(name)
	if err != nil {
		panic(fmt.Errorf("include: template %q was not found: %s", name, err))
	}

	st.newScope()
	st.blocks = t.processedBlocks

	Root := t.Root
	if t.extends != nil {
		Root = t.extends.Root
	}

	if context != nil {
		c := st.context
		st.context = reflect.ValueOf(context)
		st.executeList(Root)
		st.context = c
	} else {
		st.executeList(Root)
	}

	st.releaseScope()
}

// Set sets variable ${name} in the current template scope
func (state *Runtime) Set(name string, val interface{}) {
	state.setValue(name, reflect.ValueOf(val))
}

func (state *Runtime) setValue(name string, val reflect.Value) bool {
	sc := state.scope
	initial := sc

	// try to resolve variables in the current scope
	_, ok := sc.variables[name]

	// if not found walks parent scopes
	for !ok && sc.parent != nil {
		sc = sc.parent
		_, ok = sc.variables[name]
	}

	if ok {
		sc.variables[name] = val
		return false
	}

	for initial.variables == nil && initial.parent != nil {
		initial = initial.parent
	}

	if initial.variables != nil {
		sc.variables[name] = val
		return false
	}
	return true
}

// Resolve resolves a value from the execution context
func (state *Runtime) Resolve(name string) reflect.Value {

	if name == "." {
		return state.context
	}

	sc := state.scope
	// try to resolve variables in the current scope
	vl, ok := sc.variables[name]
	// if not found walks parent scopes
	for !ok && sc.parent != nil {
		sc = sc.parent
		vl, ok = sc.variables[name]
	}

	// if not found check globals
	if !ok {
		state.set.gmx.RLock()
		vl, ok = state.set.globals[name]
		state.set.gmx.RUnlock()
		// not found check defaultVariables
		if !ok {
			vl, ok = defaultVariables[name]
		}
	}
	return vl
}

func (st *Runtime) recover(err *error) {
	// reset state scope and context just to be safe (they might not be cleared properly if there was a panic while using the state)
	st.scope = &scope{}
	st.context = reflect.Value{}
	pool_State.Put(st)
	if recovered := recover(); recovered != nil {
		var is bool
		if _, is = recovered.(runtime.Error); is {
			panic(recovered)
		}
		*err, is = recovered.(error)
		if !is {
			panic(recovered)
		}
	}
}

func (st *Runtime) executeSet(left Expression, right reflect.Value, isdefault bool) {
	if isdefault == true && st.evalDefaultPrimaryExpression(left) == false {
		return
	}
	typ := left.Type()
	if typ == NodeIdentifier {
		st.setValue(left.(*IdentifierNode).Ident, right)
		return
	}
	var value reflect.Value
	var fields []string
	if typ == NodeChain {
		chain := left.(*ChainNode)
		value = st.evalPrimaryExpressionGroup(chain.Node)
		fields = chain.Field
	} else {
		fields = left.(*FieldNode).Ident
		value = st.context
	}
	lef := len(fields) - 1
	for i := 0; i < lef; i++ {
		value = getFieldOrMethodValue(fields[i], value)
		if !value.IsValid() {
			left.errorf("identifier %q is not available in the current scope", fields[i])
		}
	}

RESTART:
	switch value.Kind() {
	case reflect.Ptr:
		value = value.Elem()
		goto RESTART
	case reflect.Struct:
		value = value.FieldByName(fields[lef])
		if !value.IsValid() {
			left.errorf("identifier %q is not available in the current scope", fields[lef])
		}
		value.Set(right)
	case reflect.Map:
		value.SetMapIndex(reflect.ValueOf(&fields[lef]).Elem(), right)
	}
}

func (st *Runtime) executeSetList(set *SetNode, isdefault bool) {
	if set.IndexExprGetLookup {
		value := st.evalPrimaryExpressionGroup(set.Right[0])
		st.executeSet(set.Left[0], value, isdefault)
		if value.IsValid() {
			st.executeSet(set.Left[1], valueBoolTRUE, isdefault)
		} else {
			st.executeSet(set.Left[1], valueBoolFALSE, isdefault)
		}
	} else {
		for i := 0; i < len(set.Left); i++ {
			st.executeSet(set.Left[i], st.evalPrimaryExpressionGroup(set.Right[i]), isdefault)
		}
	}
}

func (st *Runtime) executeLet(key Expression, value reflect.Value, isdefault bool) {
	if isdefault == true && st.evalDefaultPrimaryExpression(key) == false {
		return
	}
	if st.variables == nil {
		st.variables = make(VarMap)
	}
	st.variables[key.(*IdentifierNode).Ident] = value
}

func (st *Runtime) executeLetList(set *SetNode) {
	if set.IndexExprGetLookup {
		value := st.evalPrimaryExpressionGroup(set.Right[0])

		st.executeLet(set.Left[0], value, false)

		if value.IsValid() {
			st.executeLet(set.Left[1], valueBoolTRUE, false)
		} else {
			st.executeLet(set.Left[1], valueBoolFALSE, false)
		}

	} else {
		for i := 0; i < len(set.Left); i++ {
			st.executeLet(set.Left[i], st.evalPrimaryExpressionGroup(set.Right[i]), false)
		}
	}
}

func (st *Runtime) executeYieldBlock(block *BlockNode, blockParam, yieldParam *BlockParameterList, expression Expression, content *ListNode) {

	needNewScope := len(blockParam.List) > 0 || len(yieldParam.List) > 0
	if needNewScope {
		st.newScope()
		for i := 0; i < len(yieldParam.List); i++ {
			p := &yieldParam.List[i]
			st.variables[p.Identifier] = st.evalPrimaryExpressionGroup(p.Expression)
		}
		for i := 0; i < len(blockParam.List); i++ {
			p := &blockParam.List[i]
			if _, found := st.variables[p.Identifier]; !found {
				if p.Expression == nil {
					st.variables[p.Identifier] = valueBoolFALSE
				} else {
					st.variables[p.Identifier] = st.evalPrimaryExpressionGroup(p.Expression)
				}
			}
		}
	}

	mycontent := st.content
	if content != nil {
		myscope := st.scope
		st.content = func(st *Runtime, expression Expression) {
			outscope := st.scope
			outcontent := st.content

			st.scope = myscope
			st.content = mycontent

			if expression != nil {
				context := st.context
				st.context = st.evalPrimaryExpressionGroup(expression)
				st.executeList(content)
				st.context = context
			} else {
				st.executeList(content)
			}

			st.scope = outscope
			st.content = outcontent
		}
	}

	if expression != nil {
		context := st.context
		st.context = st.evalPrimaryExpressionGroup(expression)
		st.executeList(block.List)
		st.context = context
	} else {
		st.executeList(block.List)
	}

	st.content = mycontent
	if needNewScope {
		st.releaseScope()
	}
}

type FilterType int

const (
	FilterUndefined FilterType = iota //Plain text.
	FilterFormat
)

type TextFilter struct {
	action FilterType
	value  string
	text   []byte
}

var optionText *TextFilter = NewTextFilter()

func NewTextFilter() *TextFilter {
	var ot TextFilter
	ot.Reset()
	return &ot
}

func (ot *TextFilter) isEnabled() bool {
	return ot.action != FilterUndefined
}

func (ot *TextFilter) Reset() {
	ot.action = FilterUndefined
	ot.value = ""
	ot.text = []byte{}
}

func (ot *TextFilter) SetText(src []byte) {
	ot.text = append(ot.text, src...)
}

func (ot *TextFilter) SetValue(value reflect.Value) {
	src := value.String()
	if src != "" {
		ot.action = FilterFormat
		ot.value = src
	}
}

func (ot *TextFilter) FormatOutput() []byte {
	var value interface{}
	var out []byte
	var err error

	for _, line := range strings.Split(strings.TrimSuffix(string(ot.text), "\n"), "\n") {
		mytext := line
		line = strings.Replace(line, " ", "", -1)
		line = strings.Replace(line, "\t", "", -1)
		if line != "" {
			if value, err = strconv.Atoi(mytext); err != nil {
				value, err = strconv.ParseFloat(mytext, 64)
			}
			if err != nil {
				value = mytext
			}
			out = append(out, []byte(fmt.Sprintf(ot.value, value))...)
		}
		out = append(out, '\n')
	}
	return out
}

func (st *Runtime) executeSwitch(list *ListNode, value reflect.Value) {
	var defaultNode *CaseNode = nil
	var found = false

	for i := 0; i < len(list.Nodes); i++ {
		node := list.Nodes[i]
		switch node.Type() {
		case NodeCase:
			node := node.(*CaseNode)
			if node.Expression.Type() == NodeUndefined {
				defaultNode = node
			} else {
				myvalue := st.evalPrimaryExpressionGroup(node.Expression)

				left := fmt.Sprintf("%v", value)
				right := fmt.Sprintf("%v", myvalue)

				if left == right {
					found = true
					st.executeList(node.List)
				}
			}
		}
	}
	if found == false && defaultNode != nil {
		st.executeList(defaultNode.List)
	}
}

func (st *Runtime) executeList(list *ListNode) {
	inNewSCOPE := false

	if list == nil {
		return
	}

	for i := 0; i < len(list.Nodes); i++ {
		node := list.Nodes[i]
		switch node.Type() {
		case NodeText:
			node := node.(*TextNode)
			if optionText.isEnabled() == false {
				_, err := st.Writer.Write(node.Text)
				if err != nil {
					node.error(err)
				}
			} else {
				optionText.SetText(node.Text)
			}
		case NodeAction:
			node := node.(*ActionNode)
			if node.Set != nil {
				if node.Set.Let {
					if !inNewSCOPE {
						st.newScope() //creates new scope in the back state
						inNewSCOPE = true
					}
					st.executeLetList(node.Set)
				} else {
					st.executeSetList(node.Set, false)
				}
			}
			if node.Pipe != nil {
				v, safeWriter := st.evalPipelineExpression(node.Pipe)
				if !safeWriter && v.IsValid() {
					if optionText.isEnabled() == false {
						if v.Type().Implements(rendererType) {
							v.Interface().(Renderer).Render(st)
						} else {
							_, err := fastprinter.PrintValue(st.escapeeWriter, v)
							if err != nil {
								node.error(err)
							}
						}
					} else {
						tmp := []byte(fmt.Sprintf("%v", v.Interface()))
						optionText.SetText(tmp)
					}
				}
			}
		case NodeSwitch:
			node := node.(*SwitchNode)
			value := st.evalPrimaryExpressionGroup(node.Expression)
			st.executeSwitch(node.List, value)
		case NodeFilter:
			node := node.(*FilterNode)
			value := st.evalPrimaryExpressionGroup(node.Expression)
			pos := strings.Index(node.Expression.String(), "(")
			if pos <= -1 {
				node.errorf("unexpected error")
			}
			funcname := node.Expression.String()[0:pos]

			switch funcname {
			case "format":
				optionText.SetValue(value)
			}

			st.executeList(node.List)
			out := optionText.FormatOutput()
			_, err := st.Writer.Write(out)
			if err != nil {
				node.error(err)
			}
			optionText.Reset()
		case NodeIf:
			node := node.(*IfNode)
			var isLet bool
			if node.Set != nil {
				if node.Set.Let {
					isLet = true
					st.newScope()
					st.executeLetList(node.Set)
				} else {
					st.executeSetList(node.Set, false)
				}
			}

			if castBoolean(st.evalPrimaryExpressionGroup(node.Expression)) {
				st.executeList(node.List)
			} else if node.ElseList != nil {
				st.executeList(node.ElseList)
			}
			if isLet {
				st.releaseScope()
			}
		case NodeRange:
			node := node.(*RangeNode)
			var expression reflect.Value

			isSet := node.Set != nil
			isLet := false
			isKeyVal := false

			context := st.context

			if isSet {
				isKeyVal = len(node.Set.Left) > 1
				expression = st.evalPrimaryExpressionGroup(node.Set.Right[0])
				if node.Set.Let {
					isLet = true
					st.newScope()
				}
			} else {
				expression = st.evalPrimaryExpressionGroup(node.Expression)
			}

			ranger := getRanger(expression)
			indexValue, rangeValue, end := ranger.Range()
			if !end {
				for !end {
					if isSet {
						if isLet {
							if isKeyVal {
								st.variables[node.Set.Left[0].String()] = indexValue
								st.variables[node.Set.Left[1].String()] = rangeValue
							} else {
								st.variables[node.Set.Left[0].String()] = rangeValue
							}
						} else {
							if isKeyVal {
								st.executeSet(node.Set.Left[0], indexValue, false)
								st.executeSet(node.Set.Left[1], rangeValue, false)
							} else {
								st.executeSet(node.Set.Left[0], rangeValue, false)
							}
						}
					} else {
						st.context = rangeValue
					}
					st.executeList(node.List)
					indexValue, rangeValue, end = ranger.Range()
				}
			} else if node.ElseList != nil {
				st.executeList(node.ElseList)
			}
			st.context = context
			if isLet {
				st.releaseScope()
			}
		case NodeYield:
			node := node.(*YieldNode)
			if node.IsContent {
				if st.content != nil {
					st.content(st, node.Expression)
				}
			} else {
				block, has := st.getBlock(node.Name)
				if has == false || block == nil {
					node.errorf("unresolved block %q!!", node.Name)
				}
				st.executeYieldBlock(block, block.Parameters, node.Parameters, node.Expression, node.Content)
			}
		case NodeBlock:
			node := node.(*BlockNode)
			block, has := st.getBlock(node.Name)
			if has == false {
				block = node
			}
			st.executeYieldBlock(block, block.Parameters, block.Parameters, block.Expression, block.Content)
		case NodeInclude:
			node := node.(*IncludeNode)
			var Name string

			name := st.evalPrimaryExpressionGroup(node.Name)
			if name.Type().Implements(stringerType) {
				Name = name.String()
			} else if name.Kind() == reflect.String {
				Name = name.String()
			} else {
				node.errorf("unexpected expression type %q in template yielding", getTypeString(name))
			}

			t, err := st.set.getTemplate(Name, node.TemplateName)
			if err != nil {
				node.error(err)
			} else {
				st.newScope()
				st.blocks = t.processedBlocks
				var context reflect.Value
				if node.Expression != nil {
					context = st.context
					st.context = st.evalPrimaryExpressionGroup(node.Expression)
				}
				Root := t.Root
				for t.extends != nil {
					t = t.extends
					Root = t.Root
				}
				st.executeList(Root)
				st.releaseScope()
				if node.Expression != nil {
					st.context = context
				}
			}
		}
	}
	if inNewSCOPE {
		st.releaseScope()
	}
}

var (
	valueBoolTRUE  = reflect.ValueOf(true)
	valueBoolFALSE = reflect.ValueOf(false)
)

func (st *Runtime) parseIndexExpr(baseExpression reflect.Value, indexExpression reflect.Value, indexType reflect.Type) (reflect.Value, error) {
	switch baseExpression.Kind() {
	case reflect.Map:
		key := baseExpression.Type().Key()
		if !indexType.AssignableTo(key) {
			if indexType.ConvertibleTo(key) {
				indexExpression = indexExpression.Convert(key)
			} else {
				return baseExpression, errors.New(indexType.String() + " is not assignable|convertible to map key " + key.String())
			}
		}
		return baseExpression.MapIndex(indexExpression), nil
	case reflect.Array, reflect.String, reflect.Slice:
		if canNumber(indexType.Kind()) {
			index := int(castInt64(indexExpression))
			if 0 <= index && index < baseExpression.Len() {
				return baseExpression.Index(index), nil
			}
			return baseExpression, fmt.Errorf("%s index out of range (index: %d, len: %d)", baseExpression.Kind().String(), index, baseExpression.Len())
		}
		return baseExpression, errors.New("non numeric value in index expression kind " + baseExpression.Kind().String())
	case reflect.Struct:
		if canNumber(indexType.Kind()) {
			return baseExpression.Field(int(castInt64(indexExpression))), nil
		} else if indexType.Kind() == reflect.String {
			return getFieldOrMethodValue(indexExpression.String(), baseExpression), nil
		}
		return baseExpression, errors.New("non numeric value in index expression kind " + baseExpression.Kind().String())
	case reflect.Interface:
		return st.parseIndexExpr(reflect.ValueOf(baseExpression.Interface()), indexExpression, indexType)
	}
	return baseExpression, errors.New("indexing is not supported in value type " + baseExpression.Kind().String())
}

func (st *Runtime) evalPrimaryExpressionGroup(node Expression) reflect.Value {
	switch node.Type() {
	case NodeAdditiveExpr:
		return st.evalAdditiveExpression(node.(*AdditiveExprNode))
	case NodeMultiplicativeExpr:
		return st.evalMultiplicativeExpression(node.(*MultiplicativeExprNode))
	case NodeComparativeExpr:
		return st.evalComparativeExpression(node.(*ComparativeExprNode))
	case NodeNumericComparativeExpr:
		return st.evalNumericComparativeExpression(node.(*NumericComparativeExprNode))
	case NodeLogicalExpr:
		return st.evalLogicalExpression(node.(*LogicalExprNode))
	case NodeNotExpr:
		return boolValue(!castBoolean(st.evalPrimaryExpressionGroup(node.(*NotExprNode).Expr)))
	case NodeTernaryExpr:
		node := node.(*TernaryExprNode)
		if castBoolean(st.evalPrimaryExpressionGroup(node.Boolean)) {
			return st.evalPrimaryExpressionGroup(node.Left)
		}
		return st.evalPrimaryExpressionGroup(node.Right)
	case NodeCallExpr:
		node := node.(*CallExprNode)
		baseExpr := st.evalBaseExpressionGroup(node.BaseExpr)
		if baseExpr.Kind() != reflect.Func {
			node.errorf("node %q is not func kind %q", node.BaseExpr, baseExpr.Type())
		}
		return st.evalCallExpression(baseExpr, node.Args)
	case NodeIndexExpr:
		node := node.(*IndexExprNode)

		baseExpression := st.evalPrimaryExpressionGroup(node.Base)
		indexExpression := st.evalPrimaryExpressionGroup(node.Index)
		indexType := indexExpression.Type()

		if baseExpression.Kind() == reflect.Interface {
			baseExpression = baseExpression.Elem()
		}

		if baseExpression.Kind() == reflect.Ptr {
			baseExpression = baseExpression.Elem()
		}

		ret, err := st.parseIndexExpr(baseExpression, indexExpression, indexType)
		if err != nil {
			node.errorf(err.Error())
		}
		return ret
	case NodeSliceExpr:
		node := node.(*SliceExprNode)
		baseExpression := st.evalPrimaryExpressionGroup(node.Base)

		var index, length int
		if node.Index != nil {
			indexExpression := st.evalPrimaryExpressionGroup(node.Index)
			if canNumber(indexExpression.Kind()) {
				index = int(castInt64(indexExpression))
			} else {
				node.Index.errorf("non numeric value in index expression kind %s", indexExpression.Kind().String())
			}
		}

		if node.EndIndex != nil {
			indexExpression := st.evalPrimaryExpressionGroup(node.EndIndex)
			if canNumber(indexExpression.Kind()) {
				length = int(castInt64(indexExpression))
			} else {
				node.EndIndex.errorf("non numeric value in index expression kind %s", indexExpression.Kind().String())
			}
		} else {
			length = baseExpression.Len()
		}

		return baseExpression.Slice(index, length)
	}
	return st.evalBaseExpressionGroup(node)
}

// notNil returns false when v.IsValid() == false
// or when v's kind can be nil and v.IsNil() == true
func notNil(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return !v.IsNil()
	default:
		return true
	}
}

func (st *Runtime) parseByType(baseExpression reflect.Value, indexExpression reflect.Value, indexType reflect.Type) (bool, error) {
	switch baseExpression.Kind() {
	case reflect.Map:
		key := baseExpression.Type().Key()
		if !indexType.AssignableTo(key) {
			if indexType.ConvertibleTo(key) {
				indexExpression = indexExpression.Convert(key)
			} else {
				return false, errors.New(indexType.String() + " is not assignable|convertible to map key " + key.String())
			}
		}
		return notNil(baseExpression.MapIndex(indexExpression)), nil
	case reflect.Array, reflect.String, reflect.Slice:
		if canNumber(indexType.Kind()) {
			i := int(castInt64(indexExpression))
			return i >= 0 && i < baseExpression.Len(), nil
		} else {
			return false, errors.New("non numeric value in index expression kind " + baseExpression.Kind().String())
		}
	case reflect.Struct:
		if canNumber(indexType.Kind()) {
			i := int(castInt64(indexExpression))
			return i >= 0 && i < baseExpression.NumField(), nil
		} else if indexType.Kind() == reflect.String {
			return notNil(getFieldOrMethodValue(indexExpression.String(), baseExpression)), nil
		} else {
			return false, errors.New("non numeric value in index expression kind " + baseExpression.Kind().String())
		}
	case reflect.Interface:
		return st.parseByType(reflect.ValueOf(baseExpression.Interface()), indexExpression, indexType)
	}
	return false, errors.New("indexing is not supported in value type " + baseExpression.Kind().String())
}

func (st *Runtime) isSet(node Node) bool {
	nodeType := node.Type()

	switch nodeType {
	case NodeIndexExpr:
		node := node.(*IndexExprNode)
		if !st.isSet(node.Base) {
			return false
		}

		if !st.isSet(node.Index) {
			return false
		}

		baseExpression := st.evalPrimaryExpressionGroup(node.Base)
		indexExpression := st.evalPrimaryExpressionGroup(node.Index)

		indexType := indexExpression.Type()
		if baseExpression.Kind() == reflect.Ptr || baseExpression.Kind() == reflect.Interface {
			baseExpression = baseExpression.Elem()
		}

		ret, err := st.parseByType(baseExpression, indexExpression, indexType)
		if err != nil {
			node.errorf(err.Error())
		}
		return ret

	case NodeIdentifier:
		value := st.Resolve(node.String())
		return notNil(value)
	case NodeField:
		node := node.(*FieldNode)
		resolved := st.context
		for i := 0; i < len(node.Ident); i++ {
			resolved = getFieldOrMethodValue(node.Ident[i], resolved)
			if !notNil(resolved) {
				return false
			}
		}
	case NodeChain:
		node := node.(*ChainNode)
		resolved, _ := st.evalFieldAccessExpression(node)
		return notNil(resolved)
	default:
		//todo: maybe work some edge cases
		if !(nodeType > beginExpressions && nodeType < endExpressions) {
			node.errorf("unexpected %q node in isset clause", node)
		}
	}
	return true
}

func (st *Runtime) evalNumericComparativeExpression(node *NumericComparativeExprNode) reflect.Value {
	left, right := st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right)
	isTrue := false
	kind := left.Kind()

	// if the left value is not a float and the right is, we need to promote the left value to a float before the calculation
	// this is necessary for expressions like 4*1.23
	needFloatPromotion := !isFloat(kind) && isFloat(right.Kind())

	switch node.Operator.typ {
	case itemGreat:
		if isInt(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Int()) > right.Float()
			} else {
				isTrue = left.Int() > toInt(right)
			}
		} else if isFloat(kind) {
			isTrue = left.Float() > toFloat(right)
		} else if isUint(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Uint()) > right.Float()
			} else {
				isTrue = left.Uint() > toUint(right)
			}
		} else {
			node.Left.errorf("a non numeric value in numeric comparative expression")
		}
	case itemGreatEquals:
		if isInt(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Int()) >= right.Float()
			} else {
				isTrue = left.Int() >= toInt(right)
			}
		} else if isFloat(kind) {
			isTrue = left.Float() >= toFloat(right)
		} else if isUint(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Uint()) >= right.Float()
			} else {
				isTrue = left.Uint() >= toUint(right)
			}
		} else {
			node.Left.errorf("a non numeric value in numeric comparative expression")
		}
	case itemLess:
		if isInt(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Int()) < right.Float()
			} else {
				isTrue = left.Int() < toInt(right)
			}
		} else if isFloat(kind) {
			isTrue = left.Float() < toFloat(right)
		} else if isUint(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Uint()) < right.Float()
			} else {
				isTrue = left.Uint() < toUint(right)
			}
		} else {
			node.Left.errorf("a non numeric value in numeric comparative expression")
		}
	case itemLessEquals:
		if isInt(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Int()) <= right.Float()
			} else {
				isTrue = left.Int() <= toInt(right)
			}
		} else if isFloat(kind) {
			isTrue = left.Float() <= toFloat(right)
		} else if isUint(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Uint()) <= right.Float()
			} else {
				isTrue = left.Uint() <= toUint(right)
			}
		} else {
			node.Left.errorf("a non numeric value in numeric comparative expression")
		}
	}
	return boolValue(isTrue)
}

func (st *Runtime) evalLogicalExpression(node *LogicalExprNode) reflect.Value {
	isTrue := castBoolean(st.evalPrimaryExpressionGroup(node.Left))
	if node.Operator.typ == itemAnd {
		isTrue = isTrue && castBoolean(st.evalPrimaryExpressionGroup(node.Right))
	} else {
		isTrue = isTrue || castBoolean(st.evalPrimaryExpressionGroup(node.Right))
	}
	return boolValue(isTrue)
}

func boolValue(isTrue bool) reflect.Value {
	if isTrue {
		return valueBoolTRUE
	}
	return valueBoolFALSE
}

func (st *Runtime) evalComparativeExpression(node *ComparativeExprNode) reflect.Value {
	left, right := st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right)
	if node.Operator.typ == itemNotEquals {
		return boolValue(!checkEquality(left, right))
	}
	return boolValue(checkEquality(left, right))
}

func toInt(v reflect.Value) int64 {
	kind := v.Kind()
	if isInt(kind) {
		return v.Int()
	} else if isFloat(kind) {
		return int64(v.Float())
	} else if isUint(kind) {
		return int64(v.Uint())
	} else if kind == reflect.String {
		n, e := strconv.ParseInt(v.String(), 10, 0)
		if e != nil {
			panic(e)
		}
		return n
	} else if kind == reflect.Bool {
		if v.Bool() {
			return 0
		}
		return 1
	}
	panic(fmt.Errorf("type: %q can't be converted to int64", v.Type()))
}

func toUint(v reflect.Value) uint64 {
	kind := v.Kind()
	if isUint(kind) {
		return v.Uint()
	} else if isInt(kind) {
		return uint64(v.Int())
	} else if isFloat(kind) {
		return uint64(v.Float())
	} else if kind == reflect.String {
		n, e := strconv.ParseUint(v.String(), 10, 0)
		if e != nil {
			panic(e)
		}
		return n
	} else if kind == reflect.Bool {
		if v.Bool() {
			return 0
		}
		return 1
	}
	panic(fmt.Errorf("type: %q can't be converted to uint64", v.Type()))
}

func toFloat(v reflect.Value) float64 {
	kind := v.Kind()
	if isFloat(kind) {
		return v.Float()
	} else if isInt(kind) {
		return float64(v.Int())
	} else if isUint(kind) {
		return float64(v.Uint())
	} else if kind == reflect.String {
		n, e := strconv.ParseFloat(v.String(), 0)
		if e != nil {
			panic(e)
		}
		return n
	} else if kind == reflect.Bool {
		if v.Bool() {
			return 0
		}
		return 1
	}
	panic(fmt.Errorf("type: %q can't be converted to float64", v.Type()))
}

func (st *Runtime) evalMultiplicativeExpression(node *MultiplicativeExprNode) reflect.Value {
	left, right := st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right)
	kind := left.Kind()
	// if the left value is not a float and the right is, we need to promote the left value to a float before the calculation
	// this is necessary for expressions like 4*1.23
	needFloatPromotion := !isFloat(kind) && isFloat(right.Kind())
	switch node.Operator.typ {
	case itemMul:
		if isInt(kind) {
			if needFloatPromotion {
				// do the promotion and calculates
				left = reflect.ValueOf(float64(left.Int()) * right.Float())
			} else {
				// do not need float promotion
				left = reflect.ValueOf(left.Int() * toInt(right))
			}
		} else if isFloat(kind) {
			left = reflect.ValueOf(left.Float() * toFloat(right))
		} else if isUint(kind) {
			if needFloatPromotion {
				left = reflect.ValueOf(float64(left.Uint()) * right.Float())
			} else {
				left = reflect.ValueOf(left.Uint() * toUint(right))
			}
		} else {
			node.Left.errorf("a non numeric value in multiplicative expression")
		}
	case itemDiv:
		if isInt(kind) {
			if needFloatPromotion {
				left = reflect.ValueOf(float64(left.Int()) / right.Float())
			} else {
				left = reflect.ValueOf(left.Int() / toInt(right))
			}
		} else if isFloat(kind) {
			left = reflect.ValueOf(left.Float() / toFloat(right))
		} else if isUint(kind) {
			if needFloatPromotion {
				left = reflect.ValueOf(float64(left.Uint()) / right.Float())
			} else {
				left = reflect.ValueOf(left.Uint() / toUint(right))
			}
		} else {
			node.Left.errorf("a non numeric value in multiplicative expression")
		}
	case itemMod:
		if isInt(kind) {
			left = reflect.ValueOf(left.Int() % toInt(right))
		} else if isFloat(kind) {
			left = reflect.ValueOf(int64(left.Float()) % toInt(right))
		} else if isUint(kind) {
			left = reflect.ValueOf(left.Uint() % toUint(right))
		} else {
			node.Left.errorf("a non numeric value in multiplicative expression")
		}
	}
	return left
}

func getInterfaceIntFloatAsString(src reflect.Value) (string, error) {
	value := fmt.Sprintf("%v", src.Interface())
	if _, err := strconv.Atoi(value); err == nil {
		return value, nil
	} else if _, err := strconv.ParseFloat(value, 64); err == nil {
		return value, nil
	}
	return "", errors.New("a non numeric value")
}

func (st *Runtime) evalAdditiveExpression(node *AdditiveExprNode) reflect.Value {

	isAdditive := node.Operator.typ == itemAdd
	if node.Left == nil {
		right := st.evalPrimaryExpressionGroup(node.Right)

		if rightValue, err := getInterfaceIntFloatAsString(right); err == nil {
			if rightRes, err := strconv.ParseFloat(rightValue, 64); err == nil {
				if isAdditive {
					return reflect.ValueOf(+rightRes)
				} else {
					return reflect.ValueOf(-rightRes)
				}
			}
		}

		node.Left.errorf("a non numeric value in additive expression")
	}

	left, right := st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right)
	leftValue, errLeft := getInterfaceIntFloatAsString(left)
	rightValue, errRight := getInterfaceIntFloatAsString(right)

	if errLeft == nil && errRight == nil {
		leftRes, errLeftFloat := strconv.ParseFloat(leftValue, 64)
		rightRes, errRightFloat := strconv.ParseFloat(rightValue, 64)
		if errLeftFloat == nil && errRightFloat == nil {
			if isAdditive {
				return reflect.ValueOf(leftRes + rightRes)
			} else {
				return reflect.ValueOf(leftRes - rightRes)
			}
		}
	} else {
		leftRes := fmt.Sprintf("%v", left.Interface())
		rightRes := fmt.Sprintf("%v", right.Interface())
		if isAdditive {
			return reflect.ValueOf(leftRes + rightRes)
		} else {
			node.Left.errorf("two strings in substraction")
		}
	}
	node.Left.errorf("unhandled value in additive expression")

	return left
}

func getTypeString(value reflect.Value) string {
	if value.IsValid() {
		return value.Type().String()
	}
	return "nil"
}

func (st *Runtime) evalBaseExpressionGroup(node Node) reflect.Value {
	switch node.Type() {
	case NodeNil:
		return reflect.ValueOf(nil)
	case NodeBool:
		if node.(*BoolNode).True {
			return valueBoolTRUE
		}
		return valueBoolFALSE
	case NodeString:
		return reflect.ValueOf(&node.(*StringNode).Text).Elem()
	case NodeIdentifier:
		resolved := st.Resolve(node.(*IdentifierNode).Ident)
		if !resolved.IsValid() {
			node.errorf("identifier %q is not available in the current scope %v", node, st.variables)
		}

		return resolved
	case NodeField:
		node := node.(*FieldNode)
		resolved := st.context
		for i := 0; i < len(node.Ident); i++ {
			fieldResolved := getFieldOrMethodValue(node.Ident[i], resolved)
			if !fieldResolved.IsValid() {
				node.errorf("there is no field or method %q in %s", node.Ident[i], getTypeString(resolved))
			}
			resolved = fieldResolved
		}
		return resolved
	case NodeChain:
		resolved, err := st.evalFieldAccessExpression(node.(*ChainNode))
		if err != nil {
			node.error(err)
		}
		return resolved
	case NodeNumber:
		node := node.(*NumberNode)
		if node.IsFloat {
			return reflect.ValueOf(&node.Float64).Elem()
		}

		if node.IsInt {
			return reflect.ValueOf(&node.Int64).Elem()
		}

		if node.IsUint {
			return reflect.ValueOf(&node.Uint64).Elem()
		}
	}
	node.errorf("unexpected node type %s in unary expression evaluating", node)
	return reflect.Value{}
}

func (st *Runtime) evalCallExpression(baseExpr reflect.Value, args []Expression, values ...reflect.Value) reflect.Value {

	if funcType.AssignableTo(baseExpr.Type()) {
		return baseExpr.Interface().(Func)(Arguments{runtime: st, argExpr: args, argVal: values})
	}

	i := len(args) + len(values)
	var returns []reflect.Value
	if i <= 10 {
		returns = reflect_Call10(i, st, baseExpr, args, values...)
	} else {
		returns = reflect_Call(make([]reflect.Value, i, i), st, baseExpr, args, values...)
	}

	if len(returns) == 0 {
		return reflect.Value{}
	}

	return returns[0]
}

func (st *Runtime) evalCommandExpression(node *CommandNode) (reflect.Value, bool) {
	term := st.evalPrimaryExpressionGroup(node.BaseExpr)
	if node.Call {
		if term.Kind() == reflect.Func {
			if term.Type() == safeWriterType {
				st.evalSafeWriter(term, node)
				return reflect.Value{}, true
			}
			return st.evalCallExpression(term, node.Args), false
		} else {
			node.Args[0].errorf("command %q type %s is not func", node.Args[0], term.Type())
		}
	}
	return term, false
}

func (st *Runtime) evalFieldAccessExpression(node *ChainNode) (reflect.Value, error) {
	resolved := st.evalPrimaryExpressionGroup(node.Node)
	for i := 0; i < len(node.Field); i++ {
		resolved = getFieldOrMethodValue(node.Field[i], resolved)
		if !resolved.IsValid() {
			return resolved, fmt.Errorf("there is no field or method %q in %s", node.Field[i], getTypeString(resolved))
		}
	}
	return resolved, nil
}

type escapeWriter struct {
	rawWriter  io.Writer
	safeWriter SafeWriter
}

func (w *escapeWriter) Write(b []byte) (int, error) {
	w.safeWriter(w.rawWriter, b)
	return 0, nil
}

func (st *Runtime) evalSafeWriter(term reflect.Value, node *CommandNode, v ...reflect.Value) {

	sw := &escapeWriter{rawWriter: st.Writer, safeWriter: term.Interface().(SafeWriter)}
	for i := 0; i < len(v); i++ {
		fastprinter.PrintValue(sw, v[i])
	}
	for i := 0; i < len(node.Args); i++ {
		fastprinter.PrintValue(sw, st.evalPrimaryExpressionGroup(node.Args[i]))
	}
}

func (st *Runtime) evalCommandPipeExpression(node *CommandNode, value reflect.Value) (reflect.Value, bool) {
	term := st.evalPrimaryExpressionGroup(node.BaseExpr)
	if term.Kind() == reflect.Func {
		if term.Type() == safeWriterType {
			st.evalSafeWriter(term, node, value)
			return reflect.Value{}, true
		}
		return st.evalCallExpression(term, node.Args, value), false
	} else {
		node.BaseExpr.errorf("pipe command %q type %s is not func", node.BaseExpr, term.Type())
	}
	return term, false
}

func (st *Runtime) evalDefaultPrimaryExpression(myexpr Expression) (ret bool) {
	defer func() {
		if r := recover(); r != nil {
			ret = true
		}
	}()
	st.evalPrimaryExpressionGroup(myexpr)
	return false
}

func (st *Runtime) evalPipelineExpression(node *PipeNode) (value reflect.Value, safeWriter bool) {

	for i := 0; i < len(node.Cmds); i++ {
		if strings.HasPrefix(node.Cmds[i].BaseExpr.String(), "default") {
			if i < 1 && value.IsValid() == false {
				node.errorf("wrong default order, value should be placed before")
			}
			if value.IsValid() == false && st.evalDefaultPrimaryExpression(node.Cmds[i-1].BaseExpr) == false {
				value = st.evalPrimaryExpressionGroup(node.Cmds[i-1].BaseExpr)
				node.Cmds = append(node.Cmds[:i-1], node.Cmds[i+1:]...)
			} else {
				if value.IsValid() == false {
					value = st.evalPrimaryExpressionGroup(node.Cmds[i].BaseExpr)
				}
				node.Cmds = append(node.Cmds[:i], node.Cmds[i+1:]...)
			}
			i = 0
		}
	}

	if value.IsValid() == false {
		value, safeWriter = st.evalCommandExpression(node.Cmds[0])
	}

	for i := 1; i < len(node.Cmds); i++ {
		if safeWriter {
			node.Cmds[i].errorf("unexpected command %s, writer command should be the last command", node.Cmds[i])
		}
		value, safeWriter = st.evalCommandPipeExpression(node.Cmds[i], value)
	}
	return
}

func reflect_Call(arguments []reflect.Value, st *Runtime, fn reflect.Value, args []Expression, values ...reflect.Value) []reflect.Value {
	typ := fn.Type()
	numIn := typ.NumIn()

	isVariadic := typ.IsVariadic()
	if isVariadic {
		numIn--
	}
	i, j := 0, 0

	for ; i < numIn && i < len(values); i++ {
		in := typ.In(i)
		term := values[i]
		if !term.Type().AssignableTo(in) {
			term = term.Convert(in)
		}
		arguments[i] = term
	}

	if isVariadic {
		in := typ.In(numIn).Elem()
		for ; i < len(values); i++ {
			term := values[i]
			if !term.Type().AssignableTo(in) {
				term = term.Convert(in)
			}
			arguments[i] = term
		}
	}

	for ; i < numIn && j < len(args); i, j = i+1, j+1 {
		in := typ.In(i)
		term := st.evalPrimaryExpressionGroup(args[j])
		if !term.Type().AssignableTo(in) {
			term = term.Convert(in)
		}
		arguments[i] = term
	}

	if isVariadic {
		in := typ.In(numIn).Elem()
		for ; j < len(args); i, j = i+1, j+1 {
			term := st.evalPrimaryExpressionGroup(args[j])
			if !term.Type().AssignableTo(in) {
				term = term.Convert(in)
			}
			arguments[i] = term
		}
	}
	return fn.Call(arguments[0:i])
}

func reflect_Call10(i int, st *Runtime, fn reflect.Value, args []Expression, values ...reflect.Value) []reflect.Value {
	var arguments [10]reflect.Value
	return reflect_Call(arguments[0:i], st, fn, args, values...)
}

func isUint(kind reflect.Kind) bool {
	return kind >= reflect.Uint && kind <= reflect.Uint64
}
func isInt(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Int64
}
func isFloat(kind reflect.Kind) bool {
	return kind == reflect.Float32 || kind == reflect.Float64
}

// checkEquality of two reflect values in the semantic of the jet runtime
func checkEquality(v1, v2 reflect.Value) bool {

	if !v1.IsValid() || !v2.IsValid() {
		return v1.IsValid() == v2.IsValid()
	}

	v1Type := v1.Type()
	v2Type := v2.Type()

	// fast path
	if v1Type != v2.Type() && !v2Type.AssignableTo(v1Type) && !v2Type.ConvertibleTo(v1Type) {
		return false
	}

	kind := v1.Kind()
	if isInt(kind) {
		return v1.Int() == toInt(v2)
	}
	if isFloat(kind) {
		return v1.Float() == toFloat(v2)
	}
	if isUint(kind) {
		return v1.Uint() == toUint(v2)
	}

	switch kind {
	case reflect.Bool:
		return v1.Bool() == castBoolean(v2)
	case reflect.String:
		return v1.String() == v2.String()
	case reflect.Array:
		vlen := v1.Len()
		if vlen == v2.Len() {
			return false
		}
		for i := 0; i < vlen; i++ {
			if !checkEquality(v1.Index(i), v2.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Slice:

		if v1.IsNil() != v2.IsNil() {
			return false
		}

		vlen := v1.Len()
		if vlen != v2.Len() {
			return false
		}

		if v1.CanAddr() && v2.CanAddr() && v1.Pointer() == v2.Pointer() {
			return true
		}

		for i := 0; i < vlen; i++ {
			if !checkEquality(v1.Index(i), v2.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if kind == v2.Kind() && (v1.IsNil() || v2.IsNil()) {
			return v1.IsNil() == v2.IsNil()
		}
		return checkEquality(reflect.ValueOf(v1.Interface()), reflect.ValueOf(v2.Interface()))
	case reflect.Ptr:
		return v1.Pointer() == v2.Pointer()
	case reflect.Struct:
		numField := v1.NumField()
		for i, n := 0, numField; i < n; i++ {
			if !checkEquality(v1.Field(i), v2.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for _, k := range v1.MapKeys() {
			val1 := v1.MapIndex(k)
			val2 := v2.MapIndex(k)
			if !val1.IsValid() || !val2.IsValid() || !checkEquality(v1.MapIndex(k), v2.MapIndex(k)) {
				return false
			}
		}
		return true
	case reflect.Func:
		return v1.IsNil() && v2.IsNil()
	default:
		// Normal equality suffices
		return v1.Interface() == v2.Interface()
	}
}

func castBoolean(v reflect.Value) bool {
	kind := v.Kind()
	switch kind {
	case reflect.Ptr:
		return v.IsNil() == false
	case reflect.Bool:
		return v.Bool()
	case reflect.Array:
		numItems := v.Len()
		for i, n := 0, numItems; i < n; i++ {
			if !castBoolean(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		numField := v.NumField()
		for i, n := 0, numField; i < n; i++ {
			if !castBoolean(v.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Map, reflect.Slice, reflect.String:
		return v.Len() > 0
	case reflect.Interface:
		return castBoolean(reflect.ValueOf(v.Interface()))
	default:
		if isInt(kind) {
			return v.Int() > 0
		}
		if isUint(kind) {
			return v.Uint() > 0
		}
		if isFloat(kind) {
			return v.Float() > 0
		}
	}
	return false
}

func canNumber(kind reflect.Kind) bool {
	return isInt(kind) || isUint(kind) || isFloat(kind)
}

func castInt64(v reflect.Value) int64 {
	kind := v.Kind()
	switch {
	case isInt(kind):
		return v.Int()
	case isUint(kind):
		return int64(v.Uint())
	case isFloat(kind):
		return int64(v.Float())
	}
	return 0
}

var cachedStructsMutex = sync.RWMutex{}
var cachedStructsFieldIndex = map[reflect.Type]map[string][]int{}

func getFieldOrMethodValue(key string, v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return reflect.Value{}
	}

	value := getValue(key, v)
	if value.Kind() == reflect.Interface && !value.IsNil() {
		value = value.Elem()
	}

	for dereferenceLimit := 2; value.Kind() == reflect.Ptr && dereferenceLimit >= 0; dereferenceLimit-- {
		if value.IsNil() {
			return reflect.ValueOf("")
		}
		value = reflect.Indirect(value)
	}

	return value
}

func getValue(key string, v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return reflect.Value{}
	}

	value := v.MethodByName(key)

	if value.IsValid() {
		return value
	}

	k := v.Kind()
	if k == reflect.Ptr || k == reflect.Interface {
		v = v.Elem()
		k = v.Kind()
		value = v.MethodByName(key)
		if value.IsValid() {
			return value
		}
	} else if v.CanAddr() {
		value = v.Addr().MethodByName(key)
		if value.IsValid() {
			return value
		}
	}

	if k == reflect.Struct {
		typ := v.Type()
		cachedStructsMutex.RLock()
		cache, ok := cachedStructsFieldIndex[typ]
		cachedStructsMutex.RUnlock()
		if !ok {
			cachedStructsMutex.Lock()
			if cache, ok = cachedStructsFieldIndex[typ]; !ok {
				cache = make(map[string][]int)
				buildCache(typ, cache, nil)
				cachedStructsFieldIndex[typ] = cache
			}
			cachedStructsMutex.Unlock()
		}
		if id, ok := cache[key]; ok {
			return v.FieldByIndex(id)
		}
		return reflect.Value{}
	} else if k == reflect.Map {
		return v.MapIndex(reflect.ValueOf(key))
	}
	return reflect.Value{}
}

func buildCache(typ reflect.Type, cache map[string][]int, parent []int) {
	numFields := typ.NumField()
	max := len(parent) + 1

	for i := 0; i < numFields; i++ {

		index := make([]int, max)
		copy(index, parent)
		index[len(parent)] = i

		field := typ.Field(i)
		if field.Anonymous {
			typ := field.Type
			if typ.Kind() == reflect.Struct {
				buildCache(typ, cache, index)
			}
		}
		cache[field.Name] = index
	}
}

func getRanger(v reflect.Value) Ranger {
	tuP := v.Type()
	if tuP.Implements(rangerType) {
		return v.Interface().(Ranger)
	}
	k := tuP.Kind()
	switch k {
	case reflect.Ptr, reflect.Interface:
		v = v.Elem()
		k = v.Kind()
		fallthrough
	case reflect.Slice, reflect.Array:
		sliceranger := pool_sliceRanger.Get().(*sliceRanger)
		sliceranger.i = -1
		sliceranger.len = v.Len()
		sliceranger.v = v
		return sliceranger
	case reflect.Map:
		mapranger := pool_mapRanger.Get().(*mapRanger)
		*mapranger = mapRanger{v: v, keys: v.MapKeys(), len: v.Len()}
		return mapranger
	case reflect.Chan:
		chanranger := pool_chanRanger.Get().(*chanRanger)
		*chanranger = chanRanger{v: v}
		return chanranger
	}
	panic(fmt.Errorf("type %s is not rangeable", tuP))
}

var (
	pool_sliceRanger = sync.Pool{
		New: func() interface{} {
			return new(sliceRanger)
		},
	}
	pool_mapRanger = sync.Pool{
		New: func() interface{} {
			return new(mapRanger)
		},
	}
	pool_chanRanger = sync.Pool{
		New: func() interface{} {
			return new(chanRanger)
		},
	}
)

type sliceRanger struct {
	v   reflect.Value
	len int
	i   int
}

func (s *sliceRanger) Range() (index, value reflect.Value, end bool) {
	s.i++
	index = reflect.ValueOf(s.i)
	if s.i < s.len {
		value = s.v.Index(s.i)
		return
	}
	pool_sliceRanger.Put(s)
	end = true
	return
}

type chanRanger struct {
	v reflect.Value
}

func (s *chanRanger) Range() (_, value reflect.Value, end bool) {
	value, end = s.v.Recv()
	if end {
		pool_chanRanger.Put(s)
	}
	return
}

type mapRanger struct {
	v    reflect.Value
	keys []reflect.Value
	len  int
	i    int
}

func (s *mapRanger) Range() (index, value reflect.Value, end bool) {
	if s.i < s.len {
		index = s.keys[s.i]
		value = s.v.MapIndex(index)
		s.i++
		return
	}
	end = true
	pool_mapRanger.Put(s)
	return
}
