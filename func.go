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
