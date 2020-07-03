# Breaking Changes

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