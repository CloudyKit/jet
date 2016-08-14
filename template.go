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
	"strings"
	"sync"
	"text/template"
)

// Set responsible to load and cache templates, also holds some runtime data
// passed to Runtime at evaluating time.
type Set struct {
	dirs              []string             // directories for look to template files
	templates         map[string]*Template // parsed templates
	escapee           SafeWriter           // escapee to use at runtime
	globals           VarMap               // global scope for this template set
	tmx               *sync.RWMutex        // template parsing mutex
	gmx               *sync.RWMutex        // global variables map mutex
	defaultExtensions []string
	developmentMode   bool
}

// SetDevelopmentMode set's development mode on/off, in development mode template will be recompiled on every run
func (s *Set) SetDevelopmentMode(b bool) *Set {
	s.developmentMode = b
	return s
}

func (a *Set) LookupGlobal(key string) (val interface{}, found bool) {
	a.gmx.RLock()
	val, found = a.globals[key]
	a.gmx.RUnlock()
	return
}

// AddGlobal add or set a global variable into the Set
func (s *Set) AddGlobal(key string, i interface{}) *Set {
	s.gmx.Lock()
	if s.globals == nil {
		s.globals = make(VarMap)
	}
	s.globals[key] = reflect.ValueOf(i)
	s.gmx.Unlock()
	return s
}

func (s *Set) AddGlobalFunc(key string, fn Func) *Set {
	return s.AddGlobal(key, fn)
}

// NewSet creates a new set, dir specifies a list of directories entries to search for templates
func NewSet(escapee SafeWriter, dir ...string) *Set {
	return &Set{dirs: dir, tmx: &sync.RWMutex{}, gmx: &sync.RWMutex{}, escapee: escapee, templates: make(map[string]*Template), defaultExtensions: append([]string{}, defaultExtensions...)}
}

// NewHTMLSet creates a new set, dir specifies a list of directories entries to search for templates
func NewHTMLSet(dir ...string) *Set {
	return NewSet(template.HTMLEscape, dir...)
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
		path, err := filepath.Abs(filepath.Join(paths[i], "src", path))
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

// fileExists checks if the template name exists by walking the list of template paths
// returns string with the full path of the template and bool true if the template file was found
func (s *Set) fileExists(name string) (string, bool) {
	for i := 0; i < len(s.dirs); i++ {
		fileName := path.Join(s.dirs[i], name)
		if _, err := os.Stat(fileName); err == nil {
			return fileName, true
		}
	}
	return "", false
}

// resolveName try to resolve a template name, the steps as follow
//	1. try provided path
//	2. try provided path+defaultExtensions
// ex: set.resolveName("catalog/products.list") with defaultExtensions set to []string{".html.jet",".jet"}
//	try catalog/products.list
//	try catalog/products.list.html.jet
//	try catalog/products.list.jet
func (s *Set) resolveName(name string) (newName, fileName string, foundLoaded, foundFile bool) {
	newName = name
	if _, foundLoaded = s.templates[newName]; foundLoaded {
		return
	}

	if fileName, foundFile = s.fileExists(name); foundFile {
		return
	}

	for _, extension := range s.defaultExtensions {
		newName = name + extension
		if _, foundLoaded = s.templates[newName]; foundLoaded {
			return
		}
		if fileName, foundFile = s.fileExists(newName); foundFile {
			return
		}
	}

	return
}

func (s *Set) resolveNameSibling(name, sibling string) (newName, fileName string, foundLoaded, foundFile, isRelativeName bool) {
	if sibling != "" {
		i := strings.LastIndex(sibling, "/")
		if i != -1 {
			if newName, fileName, foundLoaded, foundFile = s.resolveName(path.Join(sibling[:i+1], name)); foundFile || foundLoaded {
				isRelativeName = true
				return
			}
		}
	}
	newName, fileName, foundLoaded, foundFile = s.resolveName(name)
	return
}

// Parse parses the template, this method will link the template to the set but not the set to
func (s *Set) Parse(name, content string) (*Template, error) {
	sc := *s
	sc.developmentMode = true

	sc.tmx.RLock()
	t, err := sc.parse(name, content)
	sc.tmx.RUnlock()

	return t, err
}

func (s *Set) loadFromFile(name, fileName string) (template *Template, err error) {
	var content []byte
	if content, err = ioutil.ReadFile(fileName); err == nil {
		template, err = s.parse(name, string(content))
	}
	return
}

func (s *Set) getTemplateWhileParsing(parentName, name string) (template *Template, err error) {
	name = path.Clean(name)

	if s.developmentMode {
		if newName, fileName, foundLoaded, foundPath, _ := s.resolveNameSibling(name, parentName); foundPath {
			template, err = s.loadFromFile(newName, fileName)
		} else if foundLoaded {
			template = s.templates[newName]
		} else {
			err = fmt.Errorf("template %s can't be loaded", name)
		}
		return
	}

	if newName, fileName, foundLoaded, foundPath, isRelative := s.resolveNameSibling(name, parentName); foundPath {
		template, err = s.loadFromFile(newName, fileName)
		s.templates[newName] = template

		if !isRelative {
			s.templates[name] = template
		}
	} else if foundLoaded {
		template = s.templates[newName]
		if !isRelative && name != newName {
			s.templates[name] = template
		}
	} else {
		err = fmt.Errorf("template %s can't be loaded", name)
	}
	return
}

// getTemplate gets a template already loaded by name
func (s *Set) getTemplate(name, sibling string) (template *Template, err error) {
	name = path.Clean(name)

	if s.developmentMode {
		s.tmx.RLock()
		defer s.tmx.RUnlock()
		if newName, fileName, foundLoaded, foundFile, _ := s.resolveNameSibling(name, sibling); foundFile || foundLoaded {
			if foundFile {
				template, err = s.loadFromFile(newName, fileName)
			} else {
				template, _ = s.templates[newName]
			}
		} else {
			err = fmt.Errorf("template %s can't be loaded", name)
		}
		return
	}

	//fast path
	s.tmx.RLock()
	newName, fileName, foundLoaded, foundFile, isRelative := s.resolveNameSibling(name, sibling)

	if foundLoaded {
		template = s.templates[newName]
		s.tmx.RUnlock()
		if !isRelative && name != newName {
			// creates an alias
			s.tmx.Lock()
			if _, found := s.templates[name]; !found {
				s.templates[name] = template
			}
			s.tmx.Unlock()
		}
		return
	}
	s.tmx.RUnlock()

	//not found parses and cache
	s.tmx.Lock()
	defer s.tmx.Unlock()

	newName, fileName, foundLoaded, foundFile, isRelative = s.resolveNameSibling(name, sibling)
	if foundLoaded {
		template = s.templates[newName]
		if !isRelative && name != newName {
			// creates an alias
			if _, found := s.templates[name]; !found {
				s.templates[name] = template
			}
		}
	} else if foundFile {
		template, err = s.loadFromFile(newName, fileName)

		if !isRelative && name != newName {
			// creates an alias
			if _, found := s.templates[name]; !found {
				s.templates[name] = template
			}
		}

		s.templates[newName] = template
	} else {
		err = fmt.Errorf("template %s can't be loaded", name)
	}
	return
}

func (s *Set) GetTemplate(name string) (template *Template, err error) {
	template, err = s.getTemplate(name, "")
	return
}

func (s *Set) LoadTemplate(name, content string) (template *Template, err error) {
	if s.developmentMode {
		s.tmx.RLock()
		defer s.tmx.RUnlock()
		template, err = s.parse(name, content)
		return
	}

	//fast path
	var found bool
	s.tmx.RLock()
	if template, found = s.templates[name]; found {
		s.tmx.RUnlock()
		return
	}
	s.tmx.RUnlock()

	//not found parses and cache
	s.tmx.Lock()
	defer s.tmx.Unlock()

	if template, found = s.templates[name]; found {
		return
	}

	if template, err = s.parse(name, content); err == nil {
		s.templates[name] = template
	}

	return
}

func (t *Template) String() (template string) {
	if t.extends != nil {
		if len(t.root.Nodes) > 0 && len(t.imports) == 0 {
			template += fmt.Sprintf("{{extends %q}}", t.extends.ParseName)
		} else {
			template += fmt.Sprintf("{{extends %q}}", t.extends.ParseName)
		}
	}

	for k, _import := range t.imports {
		if t.extends == nil && k == 0 {
			template += fmt.Sprintf("{{import %q}}", _import.ParseName)
		} else {
			template += fmt.Sprintf("\n{{import %q}}", _import.ParseName)
		}
	}

	if t.extends != nil || len(t.imports) > 0 {
		if len(t.root.Nodes) > 0 {
			template += "\n" + t.root.String()
		}
	} else {
		template += t.root.String()
	}
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

func (scope VarMap) Set(name string, v interface{}) VarMap {
	scope[name] = reflect.ValueOf(v)
	return scope
}

func (scope VarMap) SetFunc(name string, v Func) VarMap {
	scope[name] = reflect.ValueOf(v)
	return scope
}

func (scope VarMap) SetWriter(name string, v SafeWriter) VarMap {
	scope[name] = reflect.ValueOf(v)
	return scope
}

// Execute executes the template in the w Writer
func (t *Template) Execute(w io.Writer, variables VarMap, data interface{}) error {
	return t.ExecuteI18N(nil, w, variables, data)
}

type Translator interface {
	Msg(key, defaultValue string) string
	Trans(format, defaultFormat string, v ...interface{}) string
}

func (t *Template) ExecuteI18N(translator Translator, w io.Writer, variables VarMap, data interface{}) (err error) {
	st := pool_State.Get().(*Runtime)
	defer st.recover(&err)

	st.blocks = t.processedBlocks
	st.translator = translator
	st.variables = variables
	st.set = t.set
	st.Writer = w

	// resolve extended template
	for t.extends != nil {
		t = t.extends
	}

	if data != nil {
		st.context = reflect.ValueOf(data)
	}

	st.executeList(t.root)
	return
}
