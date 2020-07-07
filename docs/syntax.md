# Syntax Reference

- [Comments](#comments)
- [Variables](#variables)
  - [Initialization](#initialization)
  - [Assignment](#assignment)
- [Expressions](#expressions)
  - [Identifiers](#identifiers)
  - [Indexing](#indexing)
    - [String](#string)
    - [Slice / Array](#slice--array)
    - [Map](#map)
    - [Struct](#struct)
  - [Field access](#field-access)
    - [Map](#map-1)
    - [Struct](#struct-1)
  - [Slicing](#slicing)
  - [Arithmetic](#arithmetic)
  - [String concatenation](#string-concatenation)
    - [Logical operators](#logical-operators)
  - [Ternary operator](#ternary-operator)
  - [Method calls](#method-calls)
  - [Function calls](#function-calls)
    - [Prefix syntax](#prefix-syntax)
    - [Pipelining](#pipelining)
- [Control Structures](#control-structures)
  - [if](#if)
    - [if / else](#if--else)
    - [if / else if](#if--else-if)
    - [if / else if / else](#if--else-if--else)
  - [range](#range)
    - [Slices / Arrays](#slices--arrays)
    - [Maps](#maps)
    - [Channels](#channels)
    - [else](#else)
  - [try](#try)
  - [try / catch](#try--catch)
- [Templates](#templates)
  - [include](#include)
  - [return](#return)
- [Blocks](#blocks)
  - [block](#block)
  - [yield](#yield)
  - [content](#content)
  - [Recursion](#recursion)
  - [extends](#extends)
  - [import](#import)

## Comments

Comments begin with `{*` and end with `*}` and will simply be dropped during template parsing.

    {* this is a comment *}

Comments can span multiple lines:

    {*
        none of this will be executed:
        {{ asd }}
        {{ include "./foo.jet" }}
    *}

## Variables

### Initialization

Variables have to be initialised before they can be used:

    {{ foo := "bar" }}

### Assignment

Initialised variables can be assigned a new value:

    {{ foo = "asd" }}

Variables initialised inside a template have no fixed type, so this is valid, too:

    {{ foo = 4711 }}


## Expressions

### Identifiers

Function and variable names are identifiers. Identifiers simply evaluate to the value stored for them in a variable scope, the globals, or the default variables. For example, the following are identifiers that resolve to built-in functions:

- `len`
- `isset`
- `split`

After `{{ foo := "foo" }}`, the `foo` in `{{ len(foo) }}` is an identifier expression and resolved to the string "foo".

### Indexing

Indexing expressions use `[]` syntax and evaluate to a byte in a string, an element in a slice or array, a value in a map, or a field of a struct.

#### String

    {{ s := "helloworld" }}
    {{ s[1] }} <!-- renders 101, the ASCII value of e -->

#### Slice / Array

    {{ s := slice("foo", "bar", "asd") }}
    {{ s[0] }} <!-- renders foo -->
    {{ i := 2 }}
    {{ s[i] }} <!-- renders asd -->

#### Map

    {{ m := map("foo", 123, "bar", 456) }}
    {{ m["foo"] }} <!-- renders 123 -->
    {{ bar := "bar" }}
    {{ m[bar] }} <!-- renders 456 -->

#### Struct

Assuming `user` is a Go struct value with a string field "Name":

    {{ user["Name"] }}

### Field access

Field access expressions use dot notation (`foo.bar`) and can be used with maps or structs. When the identifier in front of the `.` is omitted, the field is looked up in the current context (which will fail if the context is not a map or struct).

#### Map

    {{ m := map("foo", 123, "bar", 456) }}
    {{ m.foo }} <!-- renders 123 -->
    {{ s := slice(m, map("foo", 4711)) }}
    {{ range s }}
        {{ .foo }} <!-- renders 123, then 4711 -->
    {{ end }}

#### Struct

Assuming `user` is a Go struct value with a string field "Name":

    {{ user.Name }}

Assuming `users` is a slice of Go structs, o:

    {{ range users }}
        {{ .Name }}
    {{ end }}

### Slicing

You may re-slice a slice or array using the Go-like [start:end] syntax. The element at the `start` index will be included, the one at the `end` index will be excluded.

    {{ s := slice(6, 7, 8, 9, 10, 11) }}
    {{ sevenEightNine := s[1:4] }}

### Arithmetic

Basic arithmetic operators are supported: `+`, `-`, `*`, `/`, `%`

    {{ 1 + 2 * 3 - 4 }} <!-- will print 3 (1+6-4) -->
    {{ (1 + 2) * 3 - 4.1 }} <!-- will print 4.9 -->

### String concatenation

    {{ "HELLO" + " " + "WORLD!" }} <!-- will print "HELLO WORLD!" -->

#### Logical operators

The following operators are supported:

- `&&`: and
- `||`: or
- `!`: not
- `==`: equal
- `!=`: not equal
- `>`: greater than
- `>=`: greater than or equal (= not less than)
- `<`: less than
- `<=`: less than or equal (= not greater than)

Examples:

    {{ item == true || !item2 && item3 != "test" }}

    {{ item >= 12.5 || item < 6 }}

Logical expressions always evaluate to either `true` or `false`.

### Ternary operator

` x ? y : z` evaluates to `y` if `x` is truthy or `z` otherwise.

    <title>{{ .HasTitle ? .Title : "Title not set" }}</title>

### Method calls

You can call exported methods of Go types:

    {{ user.Rename("Peter") }}
    {{ range users }}
        {{ .FullName() }}
    {{ end }}

### Function calls

Function calls can be written using familiar C-like syntax:

    {{ len(s) }}
    {{ isset(foo, bar) }}

#### Prefix syntax

Function calls can also be written using a colon instead of parentheses:

    {{ len: s }}
    {{ isset: foo, bar }}

Note that function calls using this syntax can't be nested! This is valid: `{{ len: slice("asd", "foo") }}`, but this isn't: `{{ len: slice: "asd", "foo" }}`

#### Pipelining

Pipelining works by "piping" a value into a function as its first argument:

    {{ "123" | len}}
    {{ "FOO" | lower | len }}

Pipelines are evaluated left-to-right. This chaining syntax may be easier to read than deeply nested calls:

    {{ "123" | lower | upper | len }}

is equivalent to

    {{ len(upper(lower("123"))) }}

Inside a pipeline, functions can be enriched with additional parameters:

    {{ "hello" | repeat: 2 | len }}
    {{ "hello" | repeat(2) | len }}

Both of the above are equivalent to this:

    {{ len(repeat("hello", 2)) }}

Please note that the raw, unsafe, safeHtml or safeJs built-in escapers (or custom safe writers) need to be the last command evaluated in an action node. This means they have to come last when used in a pipeline.

    {{ "hello" | upper | raw }} <!-- valid -->
    {{ raw: "hello" }}          <!-- valid -->
    {{ raw: "hello" | upper }}  <!-- invalid (upper would be evaluated after raw) -->

## Control Structures

### if

You can branch inside templates depending on a condition using `if`:

    {{ if foo == "asd" }}
        foo is 'asd'!
    {{ end }}

#### if / else

You may provide an `else` block when using `if`:

    {{ if foo == "asd" }}
        foo is 'asd'!
    {{ else }}
        foo is something else!
    {{ end }}

#### if / else if

You can test for another condition using `else if`:

    {{ if foo == "asd" }}
        foo is 'asd'!
    {{ else if foo == 4711 }}
        foo is 4711!
    {{ end }}

This is exactly the same as this code:

    {{ if foo == "asd" }}
        foo is 'asd'!
    {{ else }}
        {{ if foo == 4711 }}
            foo is 4711!
        {{ end }}
    {{ end }}


#### if / else if / else

`if / else if / else` works, too, of course:

    {{ if foo == "asd" }}
        foo is 'asd'!
    {{ else if foo == 4711 }}
        foo is 4711!
    {{ else }}
        foo is something else!
    {{ end }}

and will do exactly the same as this:

    {{ if foo == "asd" }}
        foo is 'asd'!
    {{ else }}
        {{ if foo == 4711 }}
            foo is 4711!
        {{ else }}
            foo is something else!
        {{ end }}
    {{ end }}

### range

Use `range` to iterate over data, just like you would in Go, or how you would use a `foreach` loop in other programming languages. Inside the `range`, the context (`.`) is set to the current iteration's value:

    {{ s := slice("foo", "bar", "asd") }}
    {{ range s }}
        {{.}}
    {{ end }}

Jet provides built-in rangers for Go slices, arrays, maps, and channels. You can add your own by implementing the Ranger interface. TODO

#### Slices / Arrays

When iterating over a slice or array, Jet can give you the current iteration index:

    {{ range i := s }}
        {{i}}: {{.}}
    {{ end }}

If you want, you can have Jet assign the iteration value to another value:

    {{ range i, v := s }}
        {{i}}: {{v}}
    {{ end }}

The iteration value will then not be used as context (`.`); instead, the parent context remains available.

#### Maps

When iterating over a map, Jet can give you the key current iteration index:

    {{ m := map("foo", "bar", "asd", 123)}}
    {{ range k := m }}
        {{k}}: {{.}}
    {{ end }}

Just like with slices, you can have Jet assign the iteration value to another value:

    {{ range k, v := m }}
        {{k}}: {{v}}
    {{ end }}

The iteration value will then not be used as context (`.`); instead, the parent context remains available.

#### Channels

When iterating over a channel, you can can have Jet assign the iteration value to another value in order to keep the parent context, similar to the two-variables syntax for slices and maps:

    {{ range v := c }}
        {{v}}
    {{ end }}

It's an error to use channels together with the two-variable syntax.

#### else

`range` statements can have an `else` block which is executed if there are non values to range over (as signalled by the Ranger). For example, it will run when iterating an empty slice, array or map or a closed channel:

    {{ range searchResults }}
        {{.}}
    {{ else }}
        No results found :(
    {{ end }}

### try

If you want to attempt rendering something, but don't want Jet to crash when something goes wrong, you can use `try`:

    {{ try }}
        we're not sure if we already initialised foo,
        so the next line might fail...
        {{ foo }}
    {{ end }}

You can do anything you want inside a `try` block, even yield blocks or include other templates.

All render output generated inside the `try` block is buffered and only included in the surrounding output after execution of the entire block completed successfully. Any runtime error means no content from inside `try` is kept.

### try / catch

In case of an error inside the `try` block, you can have Jet evaluate a `catch` block:

    {{ try }}
        we're not sure if we already initialised foo,
        so the next line might fail...
        {{ foo }}
    {{ catch }}
        foo was not initialised, this is fallback content
    {{ end }}

Errors occuring inside the `catch` block are not caught and will cause execution to abort.

You can also have the error that occured assigned to a variable inside the `catch` block to log it or otherwise handle it. Since it's a Go error value, you have to call `.Error()` on it to get the error message as a string.

    {{ try }}
        we're not sure if we already initialised foo,
        so the next line might fail...
        {{ foo }}
    {{ catch err }}
        {{ log(err.Error()) }}
        uh oh, something went wrong: {{ err.Error() }}
    {{ end }}

`err` will not be available outside the `catch` block.

## Templates

### include

Including a template is similar to using partials in other template languages. All local and global variables are available to you in the included template. You can pass a context by specifying it as the last argument in the `include` statement. If you don't pass a context, the current context will be kept.

    <!-- file: "user.jet" -->
    <div class="user">
        {{ .["name"] }}: {{ .["email"] }}
    </div>

    <!-- file: "index.jet" -->
    {{ range users }}
        {{ include "./user.jet" }}
    {{ end }}

Executing `index.jet` with

    {{ users := map(
        "4243", map("name", "Peter", "email", "peter@aol.com"),
        "4534", map("name", "Bob", "email", "bob@yahoo.com")
    ) }}

gives you:

    <div class="user">
        Peter: peter@aol.com
    </div>
    <div class="user">
        Bob: bob@yahoo.com
    </div>

### return

Templates can set a value as their return value using `return`. This is only useful when the template was executed using the `exec()` built-in function, which will make the return value of a template available in another template.

`return` will **not** stop execution of the current block or template!

    <!-- file: "foo.jet" -->
    {{ f := "f" }}
    {{ o := "o" }}
    {{ return f+o+o }}

    <!-- file: "bar.jet" -->
    {{ foo := exec("./foo.jet") }}
    Hello, {{ foo }}!

The output will simply be:

    Hello, foo!

## Blocks

You can think of blocks as partials or pieces of a template that you can invoke by name.

### block

To define a block, use `block`:

    {{block copyright()}}
        <div>© ACME, Inc. 2020</div>
    {{end}}

Defining a block in a template that's being executed will also invoke it immediately. To avoid this, use `import` or `extends`. Blocks can't be named "content", "yield", or other Jet keywords.

A block definition accepts a comma-separated list of argument names, with optional defaults:

    {{ block inputField(type="text", label, id, value="", required=false) }}
        <div class="form-field">
            <label for="{{ id }}">{{ label }}</label>
            <input type="{{ type }}" value="{{ value }}" id="{{ id }}" {{ required ? "required" : "" }} />
        </div>
    {{ end }}

### yield

To invoke a previously defined block, use `yield`:

    <footer>
        {{yield copyright()}}
    </footer>

    {{yield inputField(id="firstname", label="First name", required=true)}}

The sequence of parameters is irrelevant, and parameters without a default value must be passed when yielding a block.

You can pass something to be used as context, or the current context will be passed. Given

    {{block buff()}}
        <strong>{{.}}</strong>
    {{end}}

the following invocation

    {{yield buff() "Batman"}}

will produce

    <strong>Batman</Batman>

### content

When defining a block, use the special `{{ yield content }}` statement to designate where any inner content should be rendered. Then, when you invoke the block with yield, use the keyword content at the end of the `yield`. For example:

    {{ block link(target) }}
        <a href="{{ target }}">{{ yield content }}</a>
    {{ end }}

    [...]

    {{ yield link(target="https://www.example.com") content }}
        Example Inc.
    {{ end }}

The output will be

    <a href="https://www.example.com">Example Inc.</a>

The invocating `yield` (`{{ yield link(target="https://www.example.com") content }}`) will store the content (together with the current variable scope) and the `{{ yield content }}` part will restore the variable scope and execute the content. When you pass a context during the `yield` block invocation, it will be used when executing the content as well.

Since defining a block will also invoke it, you can also define some content immediately as part of the `block` definition:

    {{ name := "Sarah" }}
    {{ block header() }}
        <div class="header">
        {{ yield content }}
        </div>
    {{ content }}
        <h1>Hey {{ name }}!</h1>
    {{ end }}

This will render something like the following at the position where the block is defined:

    <div class="header">
        <h1>Hey Sarah!</h1>
    </div>

### Recursion

You can yield a block inside its own definition:

    {{ block menu() }}
        <ul>
            {{ range . }}
                <li>{{ .Text }}{{ if len(.Children) }}{{ yield menu() .Children }}{{ end }}</li>
            {{ end }}
        </ul>
    {{ end }}

### extends

A template can extend another template using an `extends` statement followed by a template path at the very top:

    <!-- file: "content.jet" -->
    {{extends "./layout.jet"}}

In an extending template, content outside of a block definition will be discarded:

    <!-- file: "content.jet" -->
    {{extends "./layout.jet"}}
    {{block body()}}
    <main>
        This content can be yielded anywhere.
    </main>
    {{end}}
    This content will never be rendered.

The extended template then has to have yield slots to render your blocks into:

    <!-- file: "layout.jet" -->
    <!DOCTYPE html>
    <html>
    <body>
        {{yield body()}}
    </body>
    </html>

The final result will be:

    <!DOCTYPE html>
    <html>
    <body>
        <main>
            This content can be yielded anywhere.
        </main>
    </body>
    </html>

Every template can only extend one other template, and the `extends` statement has to be at the very top of the file (even above `import` statements).

Since the extending template isn't actually executed (the extended template is), the blocks defined in it don't run until you `yield` them explicitely.

### import

A template's defined blocks can be imported into another template using the `import` statement:

    <!-- file: "my_blocks.jet" -->
    {{ block body() }}
    <main>
        This content can be yielded anywhere.
    </main>
    {{ end }}

    <!-- file: "index.jet" -->
    {{ import "./my_blocks.jet" }}
    <!DOCTYPE html>
    <html>
    <body>
        {{ yield body() }}
    </body>
    </html>

Executing `index.jet` will produce:

    <!DOCTYPE html>
    <html>
    <body>
        <main>
            This content can be yielded anywhere.
        </main>
    </body>
    </html>

`import` makes all the blocks from the imported template available in the importing template. There is no way to only import (a) specific block(s).

Since the imported template isn't actually executed, the blocks defined in it don't run until you `yield` them explicitely.