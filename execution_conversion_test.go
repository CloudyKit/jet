package jet

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"testing"
)

func TestExecutionNumericScanBudget(t *testing.T) {
	for _, number := range []interface{}{int64(0), uint64(0), float64(0)} {
		vars := VarMap{}.Set("number", number).Set("text", strings.Repeat("0", 1000))
		err := runtimeTemplate(t, `{{ number * text }}`).ExecuteContext(context.Background(), ioutil.Discard, vars, nil, ExecutionOptions{MaxSteps: 100})
		if !errors.Is(err, ErrStepLimit) {
			t.Fatalf("%T: got %v", number, err)
		}
	}
}
