package jet

import (
	"fmt"
	"reflect"
)

type Arguments struct {
	runtime *Runtime
	argExpr []Expression
	argVal  []reflect.Value
}

func (a *Arguments) Get(argumentIndex int) reflect.Value {
	if argumentIndex < len(a.argVal) {
		return a.argVal[argumentIndex]
	}
	if argumentIndex < len(a.argVal)+len(a.argExpr) {
		return a.runtime.evalPrimaryExpressionGroup(a.argExpr[argumentIndex-len(a.argVal)])
	}
	return reflect.Value{}
}

func (a *Arguments) Panicf(format string, v ...interface{}) {
	panic(fmt.Errorf(format, v...))
}

func (a *Arguments) RequireNumOfArguments(funcname string, min, max int) {
	num := len(a.argExpr) + len(a.argVal)
	if num < min {
		a.Panicf("unexpected number of arguments in a call to %s", funcname)
	} else if num > max {
		a.Panicf("unexpected number of arguments in a call to %s", funcname)
	}
}

type Func func(Arguments) reflect.Value
