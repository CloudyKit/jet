package jet

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sync"
	"text/template"
)

type Set struct {
	Dirs      []string             // directories for look to template files
	templates map[string]*Template // parsed templates
	escapee   SafeWriter
	globals   Scope        // global scope for this template set
	tmx       sync.RWMutex // template parsing mutext
	gmx       sync.RWMutex // global variables map mutext
}

// AddGlobal (add|set)s a global variable to the set
func (s *Set) AddGlobal(key string, i interface{}) (val interface{}, override bool) {
	s.gmx.Lock()
	val, override = s.globals[key]
	if s.globals == nil {
		s.globals = make(Scope)
	}
	s.globals[key] = reflect.ValueOf(i)
	s.gmx.Unlock()
	return
}

// NewSet creates a new set, dir specifies a list of directories entries to search for templates
func NewSet(dir ...string) *Set {
	return &Set{Dirs: dir, templates: make(map[string]*Template)}
}

// NewHTMLSet creates a new set, dir specifies a list of directories entries to search for templates
func NewHTMLSet(dir ...string) *Set {
	return &Set{Dirs: dir, escapee: template.HTMLEscape, templates: make(map[string]*Template)}
}

// NewSafeSet creates a new set, dir specifies a list of directories entries to search for templates
func NewSafeSet(escapee SafeWriter, dir ...string) *Set {
	return &Set{Dirs: dir, escapee: escapee, templates: make(map[string]*Template)}
}

// AddPath adds a path to the directories entries
func (s *Set) AddPath(path string) {
	s.Dirs = append([]string{path}, s.Dirs...)
}

//AddGopathPath adds a path to directories
func (s *Set) AddGopathPath(path string) {
	paths := filepath.SplitList(os.Getenv("GOPATH"))
	for i := 0; i < len(paths); i++ {
		path, err := filepath.Abs(filepath.Join(paths[i], path))

		if err != nil {
			panic(errors.New("Can't add this path err: " + err.Error()))
		}

		if fstats, err := os.Stat(path); os.IsNotExist(err) == false && fstats.IsDir() {
			s.Dirs = append([]string{path}, s.Dirs...)
		}
	}
}

func (s *Set) load(name, content string) (template *Template, err error) {
	if content == "" {
		for i := 0; i < len(s.Dirs); i++ {
			fileName := path.Join(s.Dirs[i], name)
			var bytestring []byte
			bytestring, err = ioutil.ReadFile(fileName)
			if err == nil {
				content = string(bytestring)
				break
			}
		}
		if content == "" && err != nil {
			return
		}
	}

	template, err = s.parse(name, content)
	return
}

func (s *Set) loadTemplate(name, content string) (template *Template, err error) {
	var ok bool
	if template, ok = s.templates[name]; ok {
		return
	}
	template, err = s.load(name, content)
	s.templates[name] = template
	return
}

func (s *Set) GetTemplate(name string) (template *Template, ok bool) {
	s.tmx.RLock()
	template, ok = s.templates[name]
	s.tmx.RUnlock()
	return
}

func (s *Set) LoadTemplate(name, content string) (template *Template, err error) {
	var ok bool

	s.tmx.RLock()
	if template, ok = s.templates[name]; ok {
		s.tmx.RUnlock()
		return
	}

	s.tmx.RUnlock()
	s.tmx.Lock()
	defer s.tmx.Unlock()

	template, ok = s.templates[name]
	if ok && template != nil {
		return
	}

	template, err = s.load(name, content)
	s.templates[name] = template // saves the template
	return
}

func (t *Template) String() (template string) {

	if t.extends != nil {
		template += fmt.Sprintf("{{extends %q}}", t.extends.ParseName)
	}

	for _, _import := range t.imports {
		template += fmt.Sprintf("\n{{import %q}}", _import.ParseName)
	}

	template += t.root.String()
	return
}

func (t *Template) addBlocks(blocks map[string]*BlockNode) {
	if len(blocks) > 0 {
		if t.processedBlocks == nil {
			t.processedBlocks = make(map[string]*BlockNode)
		}
		for key, value := range blocks {
			t.processedBlocks[key] = value
		}
	}
}

type Scope map[string]reflect.Value

func (scope Scope) Set(name string, v interface{}) {
	scope[name] = reflect.ValueOf(v)
}

func (t *Template) Execute(w io.Writer, variables Scope, data interface{}) (err error) {
	st := pool_State.Get().(*State)
	defer st.recover(&err)

	root := t.root
	if t.extends != nil {
		root = t.extends.root
	}
	if data != nil {
		st.context = reflect.ValueOf(data)
	}

	st.blocks = t.processedBlocks

	st.set = t.set
	st.Writer = w
	st.variables = variables

	st.executeList(root)
	return
}
