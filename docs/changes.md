# Breaking Changes

## v6

When udpating from version 5 to version 6, there are breaking changes to the Go API:

- Set's LoadTemplate() method was removed

    LoadTemplate() (which used to parse and cache a template while bypassing the Set's Loader) is removed in favor of the [new in-memory Loader](https://godoc.org/github.com/CloudyKit/jet#InMemLoader) where you can add templates on-the-fly (which is also used for tests without template files). Using it together with a file Loader via a MultiLoader restores the previous functionality: having template files accessed via the Loader and purely in-memory templates on top of those. [#182](https://github.com/CloudyKit/jet/pull/182)

- Loader interface changed

    A Loader's Exists() method does not return the path to the template if it was found. Jet doesn't really care about what path the Loader implementation uses to locate the template. Jet expects the Loader to guarantee that the path it tried in Exists() to also work in calls to Open(), when the Exists() call returned true. [#183](https://github.com/CloudyKit/jet/pull/183)

- a new Cache interface was introduced

    Previously, it was impossible to control if and how long a Set caches templates it parsed after fetching them via the Loader. Now, you can pass a custom Cache implementation to have complete control over caching behavior, for example to invalidate a cached template and making a Set re-fetch it via the Loader and re-parse it. [#183](https://github.com/CloudyKit/jet/pull/183)

- new NewSet() with option functions

    The different functions used to create a Set (NewSet, NewSetLoader, NewHTMLSet, NewHTMLSetLoader()) have been removed in favor of a single NewSet() function that requires only a loader and accepts any number of configuration options in the form of option functions. When not passing any options, NewSet() will now use the HTML safe writer by default.

- SetDevelopmentMode(), Delims(), SetExtensions() converted to option functions

    The new InDevelopmentMode(), WithDelims() and WithTemplateNameExtensions() option functions replace the previous functions. This means you can't change these settings after the Set is created, which was very likely not a good idea anyway.

    If you toggle development mode after Set creation, you can now use a custom Cache to configure cache-use on the fly. Since this is all the development mode does anyway, the InDevelopmentMode() option might be removed in a future major version of Jet.

There are no breaking changes to the template language.

## v5

When updating from version 4 to version 5, there is a breaking change:

- `_` became a reserved symbol

    Version 5 uses `_` for two new features: it adds Go-like discard syntax in assignments (assigning anything to `_` will make jet skip the assignment) and to denote the [argument slot for the piped value](./syntax.md#piped-argument-slot). When assigning to `_`, Jet will still always evaluate the corresponding right-hand side of the assignment statement, i.e. you can use `_` to call a function but throw away its return value.

    When you assign (and/or use) a variable called `_` in your code, you will have to rename this variable.

## v4

When updating from version 3 to version 4, there are a few breaking changes:

- one-variable assignment in `range`

    `range x := someSlice` would set `x` to the value of the element in v3. In v4, `x` will be the index of the element. (Ranging over a channel didn't change.)
    See https://github.com/CloudyKit/jet/issues/158.

- Runtime.Set()

    In v3, Set() would initialise a new variable in the top scope if no variable with that name existed. In v4, Set() will return an error when trying to set a variable that doesn't exist. Let() now always sets a variable in the current scope (possibly shadowing an existing one in a parent scope). SetOrLet() will try to change the value of an existing variable and only initialize a new variable in the current scope, if the variable doesn't exist. LetGlobal() is like Let() but always acts on the top scope.

- new keywords `return`, `try`, `catch` and builtins `exec`, `ints`, `slice`, `array`

    `return`, `try`, `catch`, `exec`, `ints`, `slice` and `array` are now keywords or predefined identifiers. If you previously used `return`, `try` or `catch`, you will have to rename your variables. `exec`, `ints`, `slice` and `array` can technically be overwritten, but you should make sure not to name your things those words regardless.

- OSFileSystemLoader only handles a single directory

    Use loaders.Multi to load templates from multiple directories. See https://github.com/CloudyKit/jet/issues/128.

- relative paths

    Relative paths to templates are now handled correctly. See https://github.com/CloudyKit/jet/issues/127.