// Command toymodule is the engine with a module that uses every hook, for the engine's tests:
// a type pack, a semantic type, a connector, an MCP tool, an HTTP route and web pages.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/Mulholland-Inc/archive"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed app
var app embed.FS

func main() {
	pages, _ := fs.Sub(app, "app")
	archive.Register(archive.Module{
		Name: "toy",
		Pack: []byte(`{"types": {
		  "shout": {"description": "Text said loudly", "schema": {"type": "string"}, "parse": "shout", "check": "shout", "format": "shout"},
		  "note": {"description": "A note", "fields": {"text": {"type": "shout", "required": true}}}}}`),
		Types: map[string]archive.SemanticType{"shout": {
			Parse: func(in any, _ archive.Hints) ([]archive.Candidate, error) {
				s, _ := in.(string)
				return []archive.Candidate{{Value: strings.ToUpper(s), Confidence: 1}}, nil
			},
			Check: func(v any) error {
				if s, _ := v.(string); s != strings.ToUpper(s) {
					return errors.New("must be shouted")
				}
				return nil
			},
			Format: func(v any, _ string) string { return v.(string) + "!" },
		}},
		Connectors: map[string]archive.Connector{"counter": {
			Check: func(context.Context, *archive.Archive, *archive.Source, map[string]any) error { return nil },
			Sync: func(ctx context.Context, s *archive.Sync) error {
				for i := range 3 {
					if err := s.Emit(ctx, fmt.Sprint("count-", i), map[string]any{"text": fmt.Sprint("count ", i)}); err != nil {
						return err
					}
				}
				return nil
			},
		}},
		Tools: func(s *mcp.Server, a *archive.Archive) {
			archive.Tool(s, "toy_whoami", "Say who is calling.", func(ctx context.Context, _ struct{}) (any, error) {
				return map[string]string{"email": archive.ActorFrom(ctx).Email}, nil
			})
		},
		Routes: func(mux *http.ServeMux, a *archive.Archive) {
			mux.Handle("GET /api/toy/whoami", a.Authed(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				archive.WriteJSON(w, http.StatusOK, map[string]string{"email": archive.ActorFrom(r.Context()).Email}, nil)
			})))
		},
		App: &archive.App{Path: "/toy", Title: "Toy", Icon: "box", Files: pages},
	})
	archive.Main()
}
