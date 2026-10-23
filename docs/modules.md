# Building a product on the engine

The engine is a Go library, `github.com/Mulholland-Inc/archive`. `cmd/archive` is the engine on its own. A product built on it, such as a family archive, has its own `main`. That `main` registers a module and calls `archive.Main()`, so the product still ships as one binary in one container, and the engine never holds product code.

```go
package main

import "github.com/Mulholland-Inc/archive"

func main() {
	archive.Register(archive.Module{
		Name:       "family",
		Pack:       familySpec,                     // the module's own types, applied on every start
		Types:      map[string]archive.SemanticType{...}, // parse, check, format, like date or money
		Connectors: map[string]archive.Connector{...},    // source kinds, like gmail or imap
		Tools:      func(s *mcp.Server, a *archive.Archive) { archive.Tool(s, "tree", "...", f) },
		Routes:     func(mux *http.ServeMux, a *archive.Archive) { mux.Handle("GET /api/family/...", a.Authed(h)) },
		App:        &archive.App{Path: "/family", Title: "Family", Icon: "users", Files: pages},
	})
	archive.Main()
}
```

## The hooks

- **Pack** is a spec of the module's types. The first instance admin applies it on every start, including the first one after setup. It adds and changes only the pack's own types and leaves everyone else's alone. An unchanged pack makes no change.
- **Types** are semantic types: `Parse` reads what people or models write, `Check` validates a stored value, and `Format` shows it. A pack type names them in `parse`, `check` and `format`.
- **Connectors** are source kinds:
  - `Check` runs when a source is added.
  - `Sync` gets a `*archive.Sync` and calls `Emit` for each item. Items land in raw and, when the source has a type, go through intake.
  - Sources of a module's kind follow the same rules as gmail or imap. Only `external` and `folder` are reserved for the instance admin.
- **Tools** add MCP tools. `archive.Tool` runs each one as the person or token calling it, with the engine's error handling.
- **Routes** add HTTP routes. `a.Authed` makes a route need a signed-in caller, who is available as `archive.ActorFrom(ctx)`. `archive.WriteJSON` answers in the engine's format. Every read and write through `*archive.Archive` goes through the same permissions and row-level security as the engine's own routes.
- **App** serves static pages under `Path`. The dashboard lists them in its sidebar from `GET /api/apps`.

`src/testdata/toymodule` uses every hook. `e2e_module_test.go` drives each one through a real server.
