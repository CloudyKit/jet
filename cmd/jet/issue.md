Given the following short program, which renders :

    package main
    
    import (
    	"bytes"
    	"github.com/CloudyKit/jet"
    )
    
    func main() {
    	view := jet.NewHTMLSet("./")
    	tpl, err := view.GetTemplate("test/parent.jet")
    	if err != nil {
    		panic(err)
    	}
    	var ret bytes.Buffer
    	err = tpl.Execute(&ret, nil, nil)
    	if err != nil {
    		panic(err)
    	}
    	println(ret.String())
    }
