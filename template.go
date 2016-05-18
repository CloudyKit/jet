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

// Set responsible to load and cache templates, also holds some runtime data
// passed to Runtime at evaluating time.
type Set struct {
	dirs      []string             // directories for look to template files
	templates map[string]*Template // parsed templates
	escapee   SafeWriter           // escapee to use at runtime
	globals   VarMap               // global scope for this template set
	tmx       sync.RWMutex         // template parsing mutex
	gmx       sync.RWMutex         // global variables map mutex
}

// AddGlobal add or set a global variable into the Set
func (s *Set) AddGlobal(key string, i interface{}) (val interface{}, override bool) {
	s.gmx.Lock()
	if s.globals == nil {
		s.globals = make(VarMap)
	} else {
		val, override = s.globals[key]
	}
	s.globals[key] = reflect.ValueOf(i)
	s.gmx.Unlock()
	return
}

// NewSet creates a new set, dir specifies a list of directories entries to search for templates
func NewSet(dir ...string) *Set {
	return &Set{dirs: dir, templates: make(map[string]*Template)}
}

// NewHTMLSet creates a new set, dir specifies a list of directories entries to search for templates
func NewHTMLSet(dir ...string) *Set {
	return &Set{dirs: dir, escapee: template.HTMLEscape, templates: make(map[string]*Template)}
}

// NewSafeSet creates a new set, dir specifies a list of directories entries to search for templates
func NewSafeSet(escapee SafeWriter, dir ...string) *Set {
	return &Set{dirs: dir, escapee: escapee, templates: make(map[string]*Template)}
}

// AddPath add path to the lookup list, when loading a template the Set will
// look into the lookup list for the file matching the provided name.
func (s *Set) AddPath(path string) {
	s.dirs = append([]string{path}, s.dirs...)
}

// AddGopathPath add path based on GOPATH env to the lookup list, when loading a template the Set will
// look into the lookup list for the file matching the provided name.
func (s *Set) AddGopathPath(path string) {
	paths := filepath.SplitList(os.Getenv("GOPATH"))
	for i := 0; i < len(paths); i++ {
		path, err := filepath.Abs(filepath.Join(paths[i], path))

		if err != nil {
			panic(errors.New("Can't add this path err: " + err.Error()))
		}

		if fstats, err := os.Stat(path); os.IsNotExist(err) == false && fstats.IsDir() {
			s.AddPath(path)
			return
		}
	}

	if fstats, err := os.Stat(path); os.IsNotExist(err) == false && fstats.IsDir() {
		s.AddPath(path)
	}
}

// load loads the template by name, if content is provided template Set will not
// look in the file system and will parse the content string
func (s *Set) load(name, content string) (template *Template, err error) {
	if content == "" {
		for i := 0; i < len(s.dirs); i++ {
			fileName := path.Join(s.dirs[i], name)
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

// loadTemplate is used to load a template while parsing a template, since set is already
// locked previously we can't lock again.
func (s *Set) loadTemplate(name, content string) (template *Template, err error) {
	var ok bool
	if template, ok = s.templates[name]; ok {
		return
	}
	template, err = s.load(name, content)
	s.templates[name] = template
	return
}

// getTemplate gets a template already loaded by name
func (s *Set) getTemplate(name string) (template *Template, ok bool) {
	s.tmx.RLock()
	template, ok = s.templates[name]
	s.tmx.RUnlock()
	return
}

// GetTemplate calls LoadTemplate and returns the template, template is already loaded return it, if
// not load, cache and return
func (s *Set) GetTemplate(name string) (*Template, error) {
	return s.LoadTemplate(name, "")
}

// LoadTemplate loads a template by name, and caches the template in the set, if content is provided
// content will be parsed instead of file
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

type VarMap map[string]reflect.Value

func (scope VarMap) Set(name string, v interface{}) {
	scope[name] = reflect.ValueOf(v)
}

// Execute executes the template in the w Writer
func (t *Template) Execute(w io.Writer, variables VarMap, data interface{}) (err error) {
	st := pool_State.Get().(*Runtime)
	defer st.recover(&err)

	if data != nil {
		st.context = reflect.ValueOf(data)
	}

	st.blocks = t.processedBlocks
	st.set = t.set

	st.variables = variables
	st.Writer = w

	// resolve extended template
	for t.extends != nil {
		t = t.extends
	}

	// execute the extended root
	st.executeList(t.root)
	return
}
