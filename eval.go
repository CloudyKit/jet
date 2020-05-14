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
	return indirectEface(vl)
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

func (st *Runtime) executeSet(left Expression, right reflect.Value) {
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
		var err error
		value, err = resolveIndex(value, reflect.ValueOf(fields[i]))
		if err != nil {
			left.errorf("%v", err)
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

func (st *Runtime) executeSetList(set *SetNode) {
	if set.IndexExprGetLookup {
		value := st.evalPrimaryExpressionGroup(set.Right[0])
		st.executeSet(set.Left[0], value)
		if value.IsValid() {
			st.executeSet(set.Left[1], valueBoolTRUE)
		} else {
			st.executeSet(set.Left[1], valueBoolFALSE)
		}
	} else {
		for i := 0; i < len(set.Left); i++ {
			st.executeSet(set.Left[i], st.evalPrimaryExpressionGroup(set.Right[i]))
		}
	}
}

func (st *Runtime) executeLetList(set *SetNode) {
	if set.IndexExprGetLookup {
		value := st.evalPrimaryExpressionGroup(set.Right[0])

		st.variables[set.Left[0].(*IdentifierNode).Ident] = value

		if value.IsValid() {
			st.variables[set.Left[1].(*IdentifierNode).Ident] = valueBoolTRUE
		} else {
			st.variables[set.Left[1].(*IdentifierNode).Ident] = valueBoolFALSE
		}

	} else {
		for i := 0; i < len(set.Left); i++ {
			st.variables[set.Left[i].(*IdentifierNode).Ident] = st.evalPrimaryExpressionGroup(set.Right[i])
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

func (st *Runtime) executeList(list *ListNode) reflect.Value {
	inNewSCOPE := false
	defer func() {
		if inNewSCOPE {
			st.releaseScope()
		}
	}()

	returnValue := reflect.Value{}

	for i := 0; i < len(list.Nodes); i++ {
		node := list.Nodes[i]
		switch node.Type() {

		case NodeText:
			node := node.(*TextNode)
			_, err := st.Writer.Write(node.Text)
			if err != nil {
				node.error(err)
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
					st.executeSetList(node.Set)
				}
			}
			if node.Pipe != nil {
				v, safeWriter := st.evalPipelineExpression(node.Pipe)
				if !safeWriter && v.IsValid() {
					if v.Type().Implements(rendererType) {
						v.Interface().(Renderer).Render(st)
					} else {
						_, err := fastprinter.PrintValue(st.escapeeWriter, v)
						if err != nil {
							node.error(err)
						}
					}
				}
			}
		case NodeIf:
			node := node.(*IfNode)
			var isLet bool
			if node.Set != nil {
				if node.Set.Let {
					isLet = true
					st.newScope()
					st.executeLetList(node.Set)
				} else {
					st.executeSetList(node.Set)
				}
			}

			if isTrue(st.evalPrimaryExpressionGroup(node.Expression)) {
				returnValue = st.executeList(node.List)
			} else if node.ElseList != nil {
				returnValue = st.executeList(node.ElseList)
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
				for !end && !returnValue.IsValid() {
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
								st.executeSet(node.Set.Left[0], indexValue)
								st.executeSet(node.Set.Left[1], rangeValue)
							} else {
								st.executeSet(node.Set.Left[0], rangeValue)
							}
						}
					} else {
						st.context = rangeValue
					}
					returnValue = st.executeList(node.List)
					indexValue, rangeValue, end = ranger.Range()
				}
			} else if node.ElseList != nil {
				returnValue = st.executeList(node.ElseList)
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
				returnValue = st.executeList(Root)
				st.releaseScope()
				if node.Expression != nil {
					st.context = context
				}
			}
		case NodeReturn:
			node := node.(*ReturnNode)
			returnValue = st.evalPrimaryExpressionGroup(node.Value)
		}
	}

	return returnValue
}

var (
	valueBoolTRUE  = reflect.ValueOf(true)
	valueBoolFALSE = reflect.ValueOf(false)
)

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
		return reflect.ValueOf(!isTrue(st.evalPrimaryExpressionGroup(node.(*NotExprNode).Expr)))
	case NodeTernaryExpr:
		node := node.(*TernaryExprNode)
		if isTrue(st.evalPrimaryExpressionGroup(node.Boolean)) {
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
		base := st.evalPrimaryExpressionGroup(node.Base)
		index := st.evalPrimaryExpressionGroup(node.Index)

		resolved, err := resolveIndex(base, index)
		if err != nil {
			node.error(err)
		}
		return resolved
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

func (st *Runtime) isSet(node Node) bool {
	nodeType := node.Type()

	switch nodeType {
	case NodeIndexExpr:
		node := node.(*IndexExprNode)
		if !st.isSet(node.Base) || !st.isSet(node.Index) {
			return false
		}

		base := st.evalPrimaryExpressionGroup(node.Base)
		index := st.evalPrimaryExpressionGroup(node.Index)

		resolved, err := resolveIndex(base, index)
		return err == nil && notNil(resolved)
	case NodeIdentifier:
		value := st.Resolve(node.String())
		return notNil(value)
	case NodeField:
		node := node.(*FieldNode)
		resolved := st.context
		for i := 0; i < len(node.Ident); i++ {
			var err error
			resolved, err = resolveIndex(resolved, reflect.ValueOf(node.Ident[i]))
			if err != nil || !notNil(resolved) {
				return false
			}
		}
	case NodeChain:
		node := node.(*ChainNode)
		resolved, err := st.evalChainNodeExpression(node)
		return err == nil && notNil(resolved)
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
	return reflect.ValueOf(isTrue)
}

func (st *Runtime) evalLogicalExpression(node *LogicalExprNode) reflect.Value {
	truthy := isTrue(st.evalPrimaryExpressionGroup(node.Left))
	if node.Operator.typ == itemAnd {
		truthy = truthy && isTrue(st.evalPrimaryExpressionGroup(node.Right))
	} else {
		truthy = truthy || isTrue(st.evalPrimaryExpressionGroup(node.Right))
	}
	return reflect.ValueOf(truthy)
}

func (st *Runtime) evalComparativeExpression(node *ComparativeExprNode) reflect.Value {
	left, right := st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right)
	equal := checkEquality(left, right)
	if node.Operator.typ == itemNotEquals {
		return reflect.ValueOf(!equal)
	}
	return reflect.ValueOf(equal)
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

func (st *Runtime) evalAdditiveExpression(node *AdditiveExprNode) reflect.Value {

	isAdditive := node.Operator.typ == itemAdd
	if node.Left == nil {
		right := st.evalPrimaryExpressionGroup(node.Right)
		kind := right.Kind()
		// todo: optimize
		// todo:
		if isInt(kind) {
			if isAdditive {
				return reflect.ValueOf(+right.Int())
			} else {
				return reflect.ValueOf(-right.Int())
			}
		} else if isUint(kind) {
			if isAdditive {
				return right
			} else {
				return reflect.ValueOf(-int64(right.Uint()))
			}
		} else if isFloat(kind) {
			if isAdditive {
				return reflect.ValueOf(+right.Float())
			} else {
				return reflect.ValueOf(-right.Float())
			}
		}
		node.Left.errorf("additive expression: right side %s (%s) is not a numeric value (left is nil)", node.Right, getTypeString(right))
	}

	left, right := st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right)
	kind := left.Kind()
	// if the left value is not a float and the right is, we need to promote the left value to a float before the calculation
	// this is necessary for expressions like 4+1.23
	needFloatPromotion := !isFloat(kind) && kind != reflect.String && isFloat(right.Kind())
	if needFloatPromotion {
		if isInt(kind) {
			if isAdditive {
				left = reflect.ValueOf(float64(left.Int()) + right.Float())
			} else {
				left = reflect.ValueOf(float64(left.Int()) - right.Float())
			}
		} else if isUint(kind) {
			if isAdditive {
				left = reflect.ValueOf(float64(left.Uint()) + right.Float())
			} else {
				left = reflect.ValueOf(float64(left.Uint()) - right.Float())
			}
		} else {
			node.Left.errorf("additive expression: left side (%s (%s) needs float promotion but neither int nor uint)", node.Left, getTypeString(left))
		}
	} else {
		if isInt(kind) {
			if isAdditive {
				left = reflect.ValueOf(left.Int() + toInt(right))
			} else {
				left = reflect.ValueOf(left.Int() - toInt(right))
			}
		} else if isFloat(kind) {
			if isAdditive {
				left = reflect.ValueOf(left.Float() + toFloat(right))
			} else {
				left = reflect.ValueOf(left.Float() - toFloat(right))
			}
		} else if isUint(kind) {
			if isAdditive {
				left = reflect.ValueOf(left.Uint() + toUint(right))
			} else {
				left = reflect.ValueOf(left.Uint() - toUint(right))
			}
		} else if kind == reflect.String {
			if isAdditive {
				left = reflect.ValueOf(left.String() + fmt.Sprint(right))
			} else {
				node.Right.errorf("minus signal is not allowed with strings")
			}
		} else {
			node.Left.errorf("additive expression: left side %s (%s) is not a numeric value", node.Left, getTypeString(left))
		}
	}

	return left
}

func getTypeString(value reflect.Value) string {
	if value.IsValid() {
		return value.Type().String()
	}
	return "<invalid>"
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
			node.errorf("identifier '%s' is not available in the current scope (%+v)", node, st.variables)
		}
		return resolved
	case NodeField:
		node := node.(*FieldNode)
		resolved := st.context
		for i := 0; i < len(node.Ident); i++ {
			field, err := resolveIndex(resolved, reflect.ValueOf(node.Ident[i]))
			if err != nil {
				node.errorf("%v", err)
			}
			if !field.IsValid() {
				node.errorf("there is no field or method '%s' in %s (.%s)", node.Ident[i], getTypeString(resolved), strings.Join(node.Ident, "."))
			}
			resolved = field
		}
		return resolved
	case NodeChain:
		resolved, err := st.evalChainNodeExpression(node.(*ChainNode))
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

func (st *Runtime) evalChainNodeExpression(node *ChainNode) (reflect.Value, error) {
	resolved := st.evalPrimaryExpressionGroup(node.Node)

	for i := 0; i < len(node.Field); i++ {
		field, err := resolveIndex(resolved, reflect.ValueOf(node.Field[i]))
		if err != nil {
			return reflect.Value{}, err
		}
		if !field.IsValid() {
			if resolved.Kind() == reflect.Map && i == len(node.Field)-1 {
				// return reflect.Zero(resolved.Type().Elem()), nil
				return reflect.Value{}, nil
			}
			return reflect.Value{}, fmt.Errorf("there is no field or method '%s' in %s (%s)", node.Field[i], getTypeString(resolved), node)
		}
		resolved = field
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

func (st *Runtime) evalPipelineExpression(node *PipeNode) (value reflect.Value, safeWriter bool) {
	value, safeWriter = st.evalCommandExpression(node.Cmds[0])
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
	v1 = indirectInterface(v1)
	v2 = indirectInterface(v2)

	if !v1.IsValid() || !v2.IsValid() {
		return v1.IsValid() == v2.IsValid()
	}

	v1Type := v1.Type()
	v2Type := v2.Type()

	// fast path
	if v1Type != v2Type && !v2Type.AssignableTo(v1Type) && !v2Type.ConvertibleTo(v1Type) {
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
		return v1.Bool() == isTrue(v2)
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
		if v1.IsNil() || v2.IsNil() {
			return v1.IsNil() == v2.IsNil()
		}
		return checkEquality(v1.Elem(), v2.Elem())
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

func isTrue(v reflect.Value) bool {
	return v.IsValid() && !v.IsZero()
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

// from text/template's exec.go:
//
// indirect returns the item at the end of indirection, and a bool to indicate
// if it's nil. If the returned bool is true, the returned value's kind will be
// either a pointer or interface.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
	}
	return v, false
}

// indirectInterface returns the concrete value in an interface value, or else v itself.
// That is, if v represents the interface value x, the result is the same as reflect.ValueOf(x):
// the fact that x was an interface value is forgotten.
func indirectInterface(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Interface {
		return v.Elem()
	}
	return v
}

// indirectEface is the same as indirectInterface, but only indirects through v if its type
// is the empty interface and its value is not nil.
func indirectEface(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Interface && v.Type().NumMethod() == 0 && !v.IsNil() {
		return v.Elem()
	}
	return v
}

// mostly copied from text/template's evalField() (exec.go):
func resolveIndex(v, index reflect.Value) (reflect.Value, error) {
	if !v.IsValid() {
		return reflect.Value{}, fmt.Errorf("there is no field or method '%s' in %s (%s)", index, v, getTypeString(v))
	}

	v, isNil := indirect(v)
	if v.Kind() == reflect.Interface && isNil {
		// Calling a method on a nil interface can't work. The
		// MethodByName method call below would panic.
		return reflect.Value{}, fmt.Errorf("nil pointer evaluating %s.%s", v.Type(), index)
	}

	// Unless it's an interface, need to get to a value of type *T to guarantee
	// we see all methods of T and *T.
	if index.Kind() == reflect.String {
		ptr := v
		if ptr.Kind() != reflect.Interface && ptr.Kind() != reflect.Ptr && ptr.CanAddr() {
			ptr = ptr.Addr()
		}
		if method := ptr.MethodByName(index.String()); method.IsValid() {
			return method, nil
		}
	}

	// It's not a method on v; so now:
	//  - if v is array/slice/string, use index as numeric index
	//  - if v is a struct, use index as field name
	//  - if v is a map, use index as key
	//  - if v is (still) a pointer, indexing will fail but we check for nil to get a useful error
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.String:
		x, err := indexArg(index, v.Len())
		if err != nil {
			return reflect.Value{}, err
		}
		return indirectEface(v.Index(x)), nil
	case reflect.Struct:
		if index.Kind() != reflect.String {
			return reflect.Value{}, fmt.Errorf("can't use %s (%s, not string) as field name in struct type %s", index, index.Type(), v.Type())
		}
		tField, ok := v.Type().FieldByName(index.String())
		if ok {
			field := v.FieldByIndex(tField.Index)
			if tField.PkgPath != "" { // field is unexported
				return reflect.Value{}, fmt.Errorf("%s is an unexported field of struct type %s", index.String(), v.Type())
			}
			return indirectEface(field), nil
		}
		return reflect.Value{}, fmt.Errorf("can't use %s as field name in struct type %s", index, v.Type())
	case reflect.Map:
		// If it's a map, attempt to use the field name as a key.
		if !index.Type().AssignableTo(v.Type().Key()) {
			return reflect.Value{}, fmt.Errorf("can't use %s (%s) as key for map of type %s", index, index.Type(), v.Type())
		}
		return indirectEface(v.MapIndex(index)), nil
	case reflect.Ptr:
		etyp := v.Type().Elem()
		if etyp.Kind() == reflect.Struct && index.Kind() == reflect.String {
			if _, ok := etyp.FieldByName(index.String()); !ok {
				// If there's no such field, say "can't evaluate"
				// instead of "nil pointer evaluating".
				break
			}
		}
		if isNil {
			return reflect.Value{}, fmt.Errorf("nil pointer evaluating %s.%s", v.Type(), index)
		}
	}
	return reflect.Value{}, fmt.Errorf("can't evaluate index %s (%s) in type %s", index, index.Type(), v.Type())
}

// from Go's text/template's funcs.go:
//
// indexArg checks if a reflect.Value can be used as an index, and converts it to int if possible.
func indexArg(index reflect.Value, cap int) (int, error) {
	var x int64
	switch index.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x = index.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		x = int64(index.Uint())
	case reflect.Float32, reflect.Float64:
		x = int64(index.Float())
	case reflect.Invalid:
		return 0, fmt.Errorf("cannot index slice/array/string with nil")
	default:
		return 0, fmt.Errorf("cannot index slice/array/string with type %s", index.Type())
	}
	if int(x) < 0 || int(x) >= cap {
		return 0, fmt.Errorf("index out of range: %d", x)
	}
	return int(x), nil
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
