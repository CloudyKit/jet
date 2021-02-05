package jet

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// dumpAll returns
//  - everything in Runtime.context
//  - everything in Runtime.variables
//  - everything in Runtime.set.globals
//  - everything in Runtime.blocks
func dumpAll(a Arguments, depth int) reflect.Value {
	var b bytes.Buffer
	var vars VarMap

	ctx := a.runtime.context
	fmt.Fprintln(&b, "Runtime.context:")
	fmt.Fprintf(&b, "\t%s %q\n", ctx.Type(), ctx)

	fmt.Fprintln(&b, "Runtime.variables:")
	vars = a.runtime.variables
	for _, name := range vars.SortedKeys() {
		val := vars[name]
		//fmt.Fprintf(&b, "\t%s: %s=%v\n", val.Type(), name, val)
		fmt.Fprintf(&b, "\t%s:=%v // %s\n", name, val, val.Type())
	}

	dumpScope(&b, a.runtime.parent, depth, 0)

	fmt.Fprintln(&b, "Runtime.set.globals:")
	vars = a.runtime.set.globals
	for _, name := range vars.SortedKeys() {
		val := vars[name]
		fmt.Fprintf(&b, "\t%s:=%v // %s\n", name, val, val.Type())
	}

	blockKeys := a.runtime.scope.sortedBlocks()
	fmt.Fprintln(&b, "Runtime.blocks:")
	for _, k := range blockKeys {
		block := a.runtime.blocks[k]
		dumpBlock(&b, block)
	}

	return reflect.ValueOf(b.String())
}

func dumpScope(w io.Writer, scope *scope, maxDepth, curDepth int) {
	if maxDepth >= curDepth || scope == nil {
		return
	}
	tabs := strings.Repeat("\t", curDepth+1)
	fmt.Fprintf(w, "%sRuntime.parent.variables, depth=%d\n", tabs, curDepth)
	vars := scope.variables
	for _, k := range vars.SortedKeys() {
		fmt.Fprintf(w, "%s%s=%q\n", tabs, k, vars[k])
	}
	dumpScope(w, scope.parent, maxDepth, curDepth+1)
}

func dumpIdentified(rnt *Runtime, ids []string) reflect.Value {
	var b bytes.Buffer
	for _, id := range ids {
		dumpFindVar(&b, rnt, id)
		dumpFindBlock(&b, rnt, id)

	}
	return reflect.ValueOf(b.String())
}

func dumpFindBlock(w io.Writer, rnt *Runtime, name string) {
	if block, ok := rnt.scope.blocks[name]; ok {
		dumpBlock(w, block)
	}
}

func dumpFindVar(w io.Writer, rnt *Runtime, name string) {
	val, err := rnt.resolve(name)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "\t%s:=%v // %s\n", name, val, val.Type())
}

func dumpBlock(w io.Writer, block *BlockNode) {
	if block == nil {
		return
	}
	fmt.Fprintf(w, "\tblock %s(%s), from %s\n", block.Name, block.Parameters.String(), block.TemplatePath)
}

func fPrintVar(w io.Writer, name string, val reflect.Value) {

}
