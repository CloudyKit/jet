package yield

import "github.com/CloudyKit/jet"

type yieldValue struct {
	Name    string
	Context interface{}
}

type Block yieldValue

func (y Block) Render(r *jet.Runtime) {
	r.YieldBlock(y.Name, y.Context)
}

type Template yieldValue

func (y Template) Render(r *jet.Runtime) {
	r.YieldTemplate(y.Name, y.Context)
}
