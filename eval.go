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
	"github.com/CloudyKit/fastprinter"
	"io"
	"reflect"
	"runtime"
	"sync"
)

var (
	rangerType     = reflect.TypeOf((*Ranger)(nil)).Elem()
	rendererType   = reflect.TypeOf((*Renderer)(nil)).Elem()
	safeWriterType = reflect.TypeOf(SafeWriter(nil))
	pool_State     = sync.Pool{
		New: func() interface{} {
			return &Runtime{scope: &scope{}, escapeeWriter: new(escapeeWriter)}
		},
	}
)

type Renderer interface {
	Render(*Runtime)
}

type RendererFunc func(*Runtime)

func (renderer RendererFunc) Render(r *Runtime) {
	renderer(r)
}

type Ranger interface {
	Range() (reflect.Value, reflect.Value, bool)
}

type escapeeWriter struct {
	Writer  io.Writer
	escapee SafeWriter
	set     *Set
}

func (w *escapeeWriter) Write(b []byte) (count int, err error) {
	if w.set.escapee == nil {
		w.Writer.Write(b)
	} else {
		w.set.escapee(w.Writer, b)
	}
	return
}

type Runtime struct {
	*escapeeWriter
	*scope
	context reflect.Value
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

func (st Runtime) YieldTemplate(name string, context interface{}) {
	t, exists := st.set.getTemplate(name)
	if !exists {
		panic(fmt.Errorf("include: template %q was not found", name))
	}

	st.newScope()
	st.blocks = t.processedBlocks
	if context != nil {
		st.context = reflect.ValueOf(context)
	}
	Root := t.root
	if t.extends != nil {
		Root = t.extends.root
	}
	st.executeList(Root)
}

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
	pool_State.Put(st)
	recovered := recover()
	if recovered != nil {
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
		value = getValue(fields[i], value)
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

func (st *Runtime) executeSetList(set *SetNode) {
	for i := 0; i < len(set.Left); i++ {
		st.executeSet(set.Left[i], st.evalPrimaryExpressionGroup(set.Right[i]))
	}
}

func (st *Runtime) executeLetList(set *SetNode) {
	for i := 0; i < len(set.Left); i++ {
		st.variables[set.Left[i].(*IdentifierNode).Ident] = st.evalPrimaryExpressionGroup(set.Right[i])
	}
}

func (st *Runtime) executeList(list *ListNode) {
	inNewSCOPE := false
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
				if !safeWriter {
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
								st.executeSet(node.Set.Left[0], indexValue)
								st.executeSet(node.Set.Left[1], rangeValue)
							} else {
								st.executeSet(node.Set.Left[0], rangeValue)
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
			block, has := st.getBlock(node.Name)

			if has == false || block == nil {
				node.errorf("unresolved block %q!!", node.Name)
			}

			if node.Expression != nil {
				context := st.context
				st.context = st.evalPrimaryExpressionGroup(node.Expression)
				st.executeList(block.List)
				st.context = context
			} else {
				st.executeList(block.List)
			}
		case NodeBlock:
			node := node.(*BlockNode)
			block, has := st.getBlock(node.Name)
			if has == false {
				block = node
			}
			if node.Expression != nil {
				context := st.context
				st.context = st.evalPrimaryExpressionGroup(node.Expression)
				st.executeList(block.List)
				st.context = context
			} else {
				st.executeList(block.List)
			}
		case NodeInclude:
			node := node.(*IncludeNode)
			t, exists := st.set.getTemplate(node.Name)
			if !exists {
				node.errorf("template %q was not found!!", node.Name)
			} else {
				st.newScope()
				st.blocks = t.processedBlocks
				if node.Expression != nil {
					st.context = st.evalPrimaryExpressionGroup(node.Expression)
				}
				Root := t.root
				if t.extends != nil {
					Root = t.extends.root
				}
				st.executeList(Root)
				st.releaseScope()
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
			node.errorf("node %q is not func", node)
		}
		return st.evalCallExpression(baseExpr, node.Args)[0]
	case NodeLenExpr:
		node := node.(*BuiltinExprNode)
		expression := st.evalPrimaryExpressionGroup(node.Args[0])

		if expression.Kind() == reflect.Ptr {
			expression = expression.Elem()
		}

		switch expression.Kind() {
		case reflect.Array, reflect.Chan, reflect.Slice, reflect.Map, reflect.String:
			return reflect.ValueOf(expression.Len())
		case reflect.Struct:
			return reflect.ValueOf(expression.NumField())
		}
		node.errorf("inválid value type %s in len builtin", expression.Type())
	case NodeIndexExpr:
		node := node.(*IndexExprNode)
		baseExpression := st.evalPrimaryExpressionGroup(node.Base)
		indexExpression := st.evalPrimaryExpressionGroup(node.Index)
		indexType := indexExpression.Type()

		if baseExpression.Kind() == reflect.Ptr {
			baseExpression = baseExpression.Elem()
		}

		switch baseExpression.Kind() {
		case reflect.Map:
			key := baseExpression.Type().Key()
			if !indexType.AssignableTo(key) {
				if indexType.ConvertibleTo(key) {
					indexExpression = indexExpression.Convert(key)
				} else {
					node.errorf("%s is not assignable|convertible to map key %s", indexType.String(), key.String())
				}
			}
			return baseExpression.MapIndex(indexExpression)
		case reflect.Array, reflect.String, reflect.Slice:
			if canNumber(indexType.Kind()) {
				return baseExpression.Index(int(castInt64(indexExpression)))
			} else {
				node.errorf("non numeric value in index expression kind %s", baseExpression.Kind().String())
			}
		case reflect.Struct:
			if canNumber(indexType.Kind()) {
				return baseExpression.Field(int(castInt64(indexExpression)))
			} else if indexType.Kind() == reflect.String {
				return getValue(indexExpression.String(), baseExpression)
			} else {
				node.errorf("non numeric value in index expression kind %s", baseExpression.Kind().String())
			}
		default:
			node.errorf("indexing is not supported in value type %s", baseExpression.Kind().String())
		}
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

	case NodeIssetExpr:
		node := node.(*BuiltinExprNode)
		for i := 0; i < len(node.Args); i++ {
			switch node.Args[i].Type() {
			case NodeIdentifier:
				if st.Resolve(node.Args[i].String()).IsValid() == false {
					return valueBoolFALSE
				}
			case NodeField:
				node := node.Args[i].(*FieldNode)
				resolved := st.context
				for i := 0; i < len(node.Ident); i++ {
					resolved = getValue(node.Ident[i], resolved)
					if !resolved.IsValid() {
						return valueBoolFALSE
					}
				}
			case NodeChain:
				node := node.Args[i].(*ChainNode)
				var value = st.evalPrimaryExpressionGroup(node.Node)
				if !value.IsValid() {
					return valueBoolFALSE
				}
				for i := 0; i < len(node.Field); i++ {
					value := getValue(node.Field[i], value)
					if !value.IsValid() {
						return valueBoolFALSE
					}
				}
			default:
				node.Args[i].errorf("unexpected %q node in isset clause", node.Args[i])
			}
		}
		return valueBoolTRUE
	}
	return st.evalBaseExpressionGroup(node)
}

func (st *Runtime) evalNumericComparativeExpression(node *NumericComparativeExprNode) reflect.Value {
	left, right := generalizeValue(st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right))
	isTrue := false
	kind := left.Kind()
	switch node.Operator.typ {
	case itemGreat:
		if isUint(kind) {
			isTrue = left.Uint() > right.Uint()
		} else if isInt(kind) {
			isTrue = left.Int() > right.Int()
		} else if isFloat(kind) {
			isTrue = left.Float() > right.Float()
		}
	case itemGreatEquals:
		if isUint(kind) {
			isTrue = left.Uint() >= right.Uint()
		} else if isInt(kind) {
			isTrue = left.Int() >= right.Int()
		} else if isFloat(kind) {
			isTrue = left.Float() >= right.Float()
		}
	case itemLess:
		if isUint(kind) {
			isTrue = left.Uint() < right.Uint()
		} else if isInt(kind) {
			isTrue = left.Int() < right.Int()
		} else if isFloat(kind) {
			isTrue = left.Float() < right.Float()
		}
	case itemLessEquals:
		if isUint(kind) {
			isTrue = left.Uint() <= right.Uint()
		} else if isInt(kind) {
			isTrue = left.Int() <= right.Int()
		} else if isFloat(kind) {
			isTrue = left.Float() <= right.Float()
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
	left, right := generalizeValue(st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right))
	return boolValue(checkEquality(left, right))
}

func (st *Runtime) evalMultiplicativeExpression(node *MultiplicativeExprNode) reflect.Value {

	left, right := generalizeValue(st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right))

	kind := left.Kind()
	switch node.Operator.typ {
	case itemMul:
		if isUint(kind) {
			left = reflect.ValueOf(left.Uint() * right.Uint())
		} else if isInt(kind) {
			left = reflect.ValueOf(left.Int() * right.Int())
		} else if isFloat(kind) {
			left = reflect.ValueOf(left.Float() * right.Float())
		}
	case itemDiv:
		if isUint(kind) {
			left = reflect.ValueOf(left.Uint() / right.Uint())
		} else if isInt(kind) {
			left = reflect.ValueOf(left.Int() / right.Int())
		} else if isFloat(kind) {
			left = reflect.ValueOf(left.Float() / right.Float())
		}
	case itemMod:
		if isUint(kind) {
			left = reflect.ValueOf(left.Uint() % right.Uint())
		} else if isInt(kind) {
			left = reflect.ValueOf(left.Int() % right.Int())
		} else if isFloat(kind) {
			left = reflect.ValueOf(int64(left.Float()) % int64(left.Float()))
		}
	}
	return left
}

func (st *Runtime) evalAdditiveExpression(node *AdditiveExprNode) reflect.Value {
	left, right := generalizeValue(st.evalPrimaryExpressionGroup(node.Left), st.evalPrimaryExpressionGroup(node.Right))
	isAdditive := node.Operator.typ == itemAdd
	kind := left.Kind()
	if isUint(kind) {
		if isAdditive {
			left = reflect.ValueOf(left.Uint() + right.Uint())
		} else {
			left = reflect.ValueOf(left.Uint() - right.Uint())
		}
	} else if isInt(kind) {
		if isAdditive {
			left = reflect.ValueOf(left.Int() + right.Int())
		} else {
			left = reflect.ValueOf(left.Int() - right.Int())
		}
	} else if isFloat(kind) {
		if isAdditive {
			left = reflect.ValueOf(left.Float() + right.Float())
		} else {
			left = reflect.ValueOf(left.Float() - right.Float())
		}
	}
	return left
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
			node.errorf("identifier %q is not available in the current scope", node)
		}
		return resolved
	case NodeField:
		node := node.(*FieldNode)
		resolved := st.context
		for i := 0; i < len(node.Ident); i++ {
			fieldResolved := getValue(node.Ident[i], resolved)
			if !fieldResolved.IsValid() {
				node.errorf("there is no field or method %q in %s", node.Ident[i], resolved.Type().String())
			}
			resolved = fieldResolved
		}
		return resolved
	case NodeChain:
		node := node.(*ChainNode)
		var value = st.evalPrimaryExpressionGroup(node.Node)
		for i := 0; i < len(node.Field); i++ {
			fieldValue := getValue(node.Field[i], value)
			if !fieldValue.IsValid() {
				node.errorf("there is no field or method %q in %s", node.Field[i], value.Type().String())
			}
			value = fieldValue
		}
		return value
	case NodeNumber:
		node := node.(*NumberNode)
		if node.IsUint {
			return reflect.ValueOf(&node.Uint64).Elem()
		}
		if node.IsInt {
			return reflect.ValueOf(&node.Int64).Elem()
		}
		if node.IsFloat {
			return reflect.ValueOf(&node.Float64).Elem()
		}
	}
	node.errorf("unexpected node type %s in unary expression evaluating", node)
	return reflect.Value{}
}

func (st *Runtime) evalCallExpression(fn reflect.Value, args []Expression, values ...reflect.Value) []reflect.Value {
	i := len(args) + len(values)
	if i <= 10 {
		return reflect_Call10(i, st, fn, args, values...)
	}
	return reflect_Call(make([]reflect.Value, i, i), st, fn, args, values...)
}

func (st *Runtime) evalCommandExpression(node *CommandNode) (reflect.Value, bool) {
	term := st.evalPrimaryExpressionGroup(node.BaseExpr)
	if node.Call {
		if term.Kind() == reflect.Func {
			if term.Type() == safeWriterType {
				st.evalSafeWriter(term, node)
				return reflect.Value{}, true
			}
			returned := st.evalCallExpression(term, node.Args)
			if len(returned) == 0 {
				return reflect.Value{}, false
			}
			return returned[0], false
		} else {
			node.Args[0].errorf("command %q type %s is not func", node.Args[0], term.Type())
		}
	}
	return term, false
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
		returned := st.evalCallExpression(term, node.Args, value)
		if len(returned) == 0 {
			return reflect.Value{}, false
		}
		return returned[0], false
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

func generalizeValue(left, right reflect.Value) (reflect.Value, reflect.Value) {
	left, right = castNumeric(left), castNumeric(right)

	leftKind := left.Kind()
	rightKind := right.Kind()

	if leftKind >= reflect.Uint &&
		leftKind <= reflect.Uint64 &&
		rightKind >= reflect.Uint &&
		rightKind <= reflect.Uint64 {
		return left, right
	}
	if leftKind >= reflect.Int &&
		leftKind <= reflect.Int64 &&
		rightKind >= reflect.Int &&
		rightKind <= reflect.Int64 {
		return left, right
	}
	if leftKind >= reflect.Float32 &&
		leftKind <= reflect.Float64 &&
		rightKind >= reflect.Float32 &&
		rightKind <= reflect.Float64 {
		return left, right
	}

	if rightKind == reflect.Float64 || rightKind == reflect.Float32 {
		return left.Convert(right.Type()), right
	}
	return left, right.Convert(left.Type())
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

func checkEquality(v1, v2 reflect.Value) bool {
	if !v1.IsValid() || !v2.IsValid() {
		return v1.IsValid() == v2.IsValid()
	}

	if v1.Type() != v2.Type() {
		return false
	}

	switch v1.Kind() {
	case reflect.Array:
		for i := 0; i < v1.Len(); i++ {
			if !checkEquality(v1.Index(i), v2.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for i := 0; i < v1.Len(); i++ {
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

func castNumeric(v reflect.Value) reflect.Value {
	kind := v.Kind()
	if isInt(kind) || isUint(kind) || isFloat(kind) {
		return v
	}
	if castBoolean(v) {
		return reflect.ValueOf(1)
	}
	return reflect.ValueOf(1)
}

var cacheStructMutex = sync.RWMutex{}
var cacheStructFieldIndex = map[reflect.Type]map[string][]int{}

func getValue(key string, v reflect.Value) reflect.Value {
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
	}

	if k == reflect.Struct {
		typ := v.Type()
		cacheStructMutex.RLock()
		cache, ok := cacheStructFieldIndex[typ]
		cacheStructMutex.RUnlock()
		if !ok {
			cacheStructMutex.Lock()
			if cache, ok = cacheStructFieldIndex[typ]; !ok {
				cache = make(map[string][]int)
				numFields := typ.NumField()
				for i := 0; i < numFields; i++ {
					field := typ.Field(i)
					cache[field.Name] = field.Index
				}
				cacheStructFieldIndex[typ] = cache
			}
			cacheStructMutex.Unlock()
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
		sliceranger.i = 0
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
	return nil
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
	index = reflect.ValueOf(&s.i).Elem()
	if s.i < s.len {
		value = s.v.Index(s.i)
		s.i++
		return
	}
	pool_sliceRanger.Put(s)
	end = true
	return
}

type chanRanger struct {
	v reflect.Value
}

func (s *chanRanger) Range() (index, value reflect.Value, end bool) {
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
