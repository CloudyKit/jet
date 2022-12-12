package jet

import (
	"fmt"
	"os"
	"reflect"
)

// exampleCustomBenchRanger satisfies the Ranger interface, generating fixed
// data.
type exampleCustomRanger struct {
	i int
}

// Type assertion to verify exampleCustomRanger satisfies the Ranger interface.
var _ Ranger = (*exampleCustomRanger)(nil)

func (ecr *exampleCustomRanger) ProvidesIndex() bool {
	// Return false if 'k' can't be filled in Range().
	return true
}

func (ecr *exampleCustomRanger) Range() (k reflect.Value, v reflect.Value, done bool) {
	if ecr.i >= 3 {
		done = true
		return
	}

	k = reflect.ValueOf(ecr.i)
	v = reflect.ValueOf(fmt.Sprintf("custom ranger %d", ecr.i))
	ecr.i += 1
	return
}

// ExampleRanger demonstrates how to write a custom template ranger.
func ExampleRanger() {
	// Error handling ignored for brevity.
	//
	// Setup template and rendering.
	loader := NewInMemLoader()
	loader.Set("template", "{{range k := ecr }}{{k}}:{{.}}; {{end}}")
	set := NewSet(loader, WithSafeWriter(nil))
	t, _ := set.GetTemplate("template")

	// Pass a custom ranger instance as the 'ecr' var.
	vars := VarMap{"ecr": reflect.ValueOf(&exampleCustomRanger{})}

	// Execute template.
	_ = t.Execute(os.Stdout, vars, nil)

	// Output: 0:custom ranger 0; 1:custom ranger 1; 2:custom ranger 2;
}
