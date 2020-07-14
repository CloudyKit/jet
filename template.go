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

// Jet is a fast and dynamic template engine for the Go programming language, set of features
// includes very fast template execution, a dynamic and flexible language, template inheritance, low number of allocations,
// special interfaces to allow even further optimizations.

package jet

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sync"
	"text/template"
)

var defaultExtensions = []string{
	"", // in case the path is given with the correct extension already
	".jet",
	".html.jet",
	".jet.html",
}

// Set is responsible to load,invoke parse and cache templates and relations
// every jet template is associated with one set.
// create a set with jet.NewSet(escapeeFn) returns a pointer to the Set
type Set struct {
	loader          Loader
	templates       map[string]*Template // parsed templates
	escapee         SafeWriter           // escapee to use at runtime
	globals         VarMap               // global scope for this template set
	tmx             *sync.RWMutex        // template parsing mutex
	gmx             *sync.RWMutex        // global variables map mutex
	extensions      []string
	developmentMode bool
	leftDelim       string
	rightDelim      string
}

// SetDevelopmentMode set's development mode on/off, in development mode template will be recompiled on every run
func (s *Set) SetDevelopmentMode(b bool) *Set {
	s.developmentMode = b
	return s
}

func (s *Set) LookupGlobal(key string) (val interface{}, found bool) {
	s.gmx.RLock()
	val, found = s.globals[key]
	s.gmx.RUnlock()
	return
}

// AddGlobal add or set a global variable into the Set
func (s *Set) AddGlobal(key string, i interface{}) *Set {
	s.gmx.Lock()
	s.globals[key] = reflect.ValueOf(i)
	s.gmx.Unlock()
	return s
}

func (s *Set) AddGlobalFunc(key string, fn Func) *Set {
	return s.AddGlobal(key, fn)
}

// NewSetLoader creates a new set with custom Loader
func NewSetLoader(escapee SafeWriter, loader Loader) *Set {
	return &Set{
		loader:     loader,
		templates:  map[string]*Template{},
		escapee:    escapee,
		globals:    VarMap{},
		tmx:        &sync.RWMutex{},
		gmx:        &sync.RWMutex{},
		extensions: append([]string{}, defaultExtensions...),
	}
}

// NewHTMLSetLoader creates a new set with custom Loader
func NewHTMLSetLoader(loader Loader) *Set {
	return NewSetLoader(template.HTMLEscape, loader)
}

// NewSet creates a new set, dirs is a list of directories to be searched for templates
func NewSet(escapee SafeWriter, dir string) *Set {
	return NewSetLoader(escapee, &OSFileSystemLoader{dir: dir})
}

// NewHTMLSet creates a new set, dirs is a list of directories to be searched for templates
func NewHTMLSet(dir string) *Set {
	return NewSet(template.HTMLEscape, dir)
}

// Delims sets the delimiters to the specified strings. Parsed templates will
// inherit the settings. Not setting them leaves them at the default: {{ or }}.
func (s *Set) Delims(left, right string) {
	s.leftDelim = left
	s.rightDelim = right
}

func (s *Set) getTemplateFromCache(path string) (t *Template, ok bool) {
	// check path with all possible extensions in cache
	for _, extension := range s.extensions {
		canonicalPath := path + extension
		if t, found := s.templates[canonicalPath]; found {
			return t, true
		}
	}
	return nil, false
}

func (s *Set) getTemplateFromLoader(path string) (t *Template, err error) {
	// check path with all possible extensions in loader
	for _, extension := range s.extensions {
		canonicalPath := path + extension
		if _, found := s.loader.Exists(canonicalPath); found {
			return s.loadFromFile(canonicalPath)
		}
	}
	return nil, fmt.Errorf("template %s could not be found", path)
}

// GetTemplate tries to find (and load, if not yet loaded) the template at the specified path.
//
// for example, GetTemplate("catalog/products.list") with extensions set to []string{"", ".html.jet",".jet"}
// will try to look for:
//     1. catalog/products.list
//     2. catalog/products.list.html.jet
//     3. catalog/products.list.jet
// in the set's templates cache, and if it can't find the template it will try to load the same paths via
// the loader, and, if parsed successfully, cache the template.
func (s *Set) GetTemplate(path string) (t *Template, err error) {
	if !s.developmentMode {
		s.tmx.RLock()
		t, found := s.getTemplateFromCache(path)
		if found {
			s.tmx.RUnlock()
			return t, nil
		}
		s.tmx.RUnlock()
	}

	t, err = s.getTemplateFromLoader(path)
	if err == nil && !s.developmentMode {
		// load template into cache
		s.tmx.Lock()
		s.templates[t.Name] = t
		s.tmx.Unlock()
	}
	return t, err
}

// same as GetTemplate, but assumes the reader already called s.tmx.RLock(), and
// doesn't cache a template when found through the loader
func (s *Set) getTemplate(path string) (t *Template, err error) {
	if !s.developmentMode {
		t, found := s.getTemplateFromCache(path)
		if found {
			return t, nil
		}
	}

	return s.getTemplateFromLoader(path)
}

func (s *Set) getSiblingTemplate(path, siblingPath string) (t *Template, err error) {
	if !filepath.IsAbs(filepath.Clean(path)) {
		siblingDir := filepath.Dir(siblingPath)
		path = filepath.Join(siblingDir, path)
	}
	return s.getTemplate(path)
}

// Parse parses the template without adding it to the set's templates cache.
func (s *Set) Parse(path, content string) (*Template, error) {
	s.tmx.RLock()
	t, err := s.parse(path, content)
	s.tmx.RUnlock()

	return t, err
}

func (s *Set) loadFromFile(path string) (template *Template, err error) {
	f, err := s.loader.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return s.parse(path, string(content))
}

func (s *Set) LoadTemplate(path, content string) (*Template, error) {
	if s.developmentMode {
		s.tmx.RLock()
		defer s.tmx.RUnlock()
		return s.parse(path, content)
	}

	// fast path (t from cache)
	s.tmx.RLock()
	if t, found := s.templates[path]; found {
		s.tmx.RUnlock()
		return t, nil
	}
	s.tmx.RUnlock()

	// not found, parse and cache
	s.tmx.Lock()
	defer s.tmx.Unlock()

	t, err := s.parse(path, content)
	if err == nil {
		s.templates[path] = t
	}
	return t, err
}

// SetExtensions sets extensions.
func (s *Set) SetExtensions(extensions []string) {
	s.extensions = extensions
}

func (t *Template) String() (template string) {
	if t.extends != nil {
		if len(t.Root.Nodes) > 0 && len(t.imports) == 0 {
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
		if len(t.Root.Nodes) > 0 {
			template += "\n" + t.Root.String()
		}
	} else {
		template += t.Root.String()
	}
	return
}

func (t *Template) addBlocks(blocks map[string]*BlockNode) {
	if len(blocks) == 0 {
		return
	}
	if t.processedBlocks == nil {
		t.processedBlocks = make(map[string]*BlockNode)
	}
	for key, value := range blocks {
		t.processedBlocks[key] = value
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

	st.executeList(t.Root)
	return
}
