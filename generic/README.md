# generic - Typed utility functions and template rendering for Go

A small library of generic utility types for Go, providing typed list
operations, pointer helpers, and a type-safe HTML template renderer.

## Installation

```bash
go get github.com/titpetric/tools/generic
```

## List

`List[T]` is a typed slice with functional operations:

```go
items := generic.NewList[string]()

// Filter returns elements matching a predicate
active := items.Filter(func(s string) bool {
    return s != ""
})

// Find returns the first match
first := items.Find(func(s string) bool {
    return strings.HasPrefix(s, "api")
})

// Get returns element at index (zero value if out of bounds)
second := items.Get(1)

// ListMap transforms elements from one type to another
lengths := generic.ListMap(items, func(s string) int {
    return len(s)
})
```

## Pointer

`Pointer[T]` returns a pointer to any value, useful for inline
initialization of optional fields:

```go
ts := generic.Pointer(time.Now())   // *time.Time
name := generic.Pointer("default")  // *string
```

## Template Renderer

`NewTemplateRenderer` creates a type-safe HTML template renderer backed
by `embed.FS`. Templates can be overridden by local files on disk.

```go
type PageData struct {
    Title string
}

func (PageData) FuncMap() template.FuncMap {
    return template.FuncMap{}
}

//go:embed templates/*
var files embed.FS

render := generic.NewTemplateRenderer[PageData](&files, "templates/_common.tpl")

// Use in an HTTP handler:
render(w, "templates/page.tpl", PageData{Title: "Hello"})
```

The renderer provides a built-in `json` template function for inspecting
data structures during development. Custom functions are supplied via the
`FuncMap()` method on the data type.

Template loading prefers local files over embedded ones, allowing
developers to override templates without recompiling.
