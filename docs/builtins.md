# Built-ins

- [Functions](#functions)
  - [From Go](#from-go)
  - [len](#len)
  - [isset](#isset)
  - [exec](#exec)
  - [ints](#ints)

## Functions

### From Go

The following functions simply expose functions from Go's standard library for convenience:

- `lower`: exposes Go's [strings.ToLower](https://golang.org/pkg/strings/#ToLower)
- `upper`: exposes Go's [strings.ToUpper](https://golang.org/pkg/strings/#ToUpper)
- `hasPrefix`: exposes Go's [strings.HasPrefix](https://golang.org/pkg/strings/#HasPrefix)
- `hasSuffix`: exposes Go's [strings.HasSuffix](https://golang.org/pkg/strings/#HasSuffix)
- `repeat`: exposes Go's [strings.Repeat](https://golang.org/pkg/strings/#Repeat)
- `replace`: exposes Go's [strings.Replace](https://golang.org/pkg/strings/#Replace)
- `split`: exposes Go's [strings.Split](https://golang.org/pkg/strings/#Split)
- `trimSpace`: exposes Go's [strings.TrimSpace](https://golang.org/pkg/strings/#TrimSpace)
- `html`: exposes Go's [html.EscapeString](https://golang.org/pkg/html/#EscapeString)
- `url`: exposes Go's [url.QueryEscape](https://golang.org/pkg/net/url/#QueryEscape)

### len

`len()` takes one argument and returns the length of a string, array, slice or map, the number of fields in a struct, or the buffer size of a channel, depending on the argument's type. (Think of it like Go's `len()` function.)

It panics if you pass a value of any type other than string, array, slice, map, struct or channel.

`len()` indirects through arbitrary layers of pointer and interface types before checking for a valid type.

### isset

`isset()` takes an arbitrary number of index, field, chain or identifier expressions and returns true if all expressions evaluate to non-nil values. It panics only when an unexpected expression type is passed in.

### exec

`exec()` takes a template path and optionally a value to use as context and executes the template with the current or specified context. It returns the last value returned using the `return` statement, or nil if no `return` statement was executed.

### ints

`ints()` takes two integers as lower and upper limit and returns a Ranger producing all the integers between them, including the lower and excluding the upper limit. It panics when the arguments can't be converted to integers or when the upper limit is not strictly greater than the lower limit.