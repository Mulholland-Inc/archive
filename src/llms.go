package archive

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// llmsHandler serves /llms.txt (llmstxt.org): the archive described for an agent, what it is,
// where its MCP server is, how a person lets an agent in, and every tool the server has, read
// from the server itself the first time it is asked, so it is never out of date.
func (a *Archive) llmsHandler(srv *mcp.Server) http.HandlerFunc {
	var once sync.Once
	var txt string
	return func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { txt = a.llms(r.Context(), srv) })
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(txt))
	}
}

func (a *Archive) llms(ctx context.Context, srv *mcp.Server) string {
	name, about := "Archive", "Archive keeps a person's or a family's things in one place, as typed records an agent can read and, where it is allowed, write."
	if b := brandOf(); b != nil {
		name, about = cmp.Or(b.Name, name), cmp.Or(b.About, about)
	}
	var tools []*mcp.Tool
	st, ct := mcp.NewInMemoryTransports()
	if ss, err := srv.Connect(ctx, st, nil); err == nil {
		defer ss.Close()
		if cs, err := mcp.NewClient(&mcp.Implementation{Name: "llms.txt"}, nil).Connect(ctx, ct, nil); err == nil {
			defer cs.Close()
			for t, err := range cs.Tools(ctx, nil) {
				if err == nil {
					tools = append(tools, t)
				}
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n> %s\n\n", name, about)
	fmt.Fprintf(&b, "## Connect\n\n")
	fmt.Fprintf(&b, "- The MCP server is %s/mcp, over Streamable HTTP. A request with no token is answered 401, naming the metadata below.\n", a.base)
	fmt.Fprintf(&b, "- Sign in with OAuth 2.1 and PKCE (S256): %s/.well-known/oauth-protected-resource/mcp and %s/.well-known/oauth-authorization-server say where. Register your client at %s/oauth/register, as a public client or with a secret.\n", a.base, a.base, a.base)
	fmt.Fprintf(&b, "- A person signs in and allows you on a page of the archive's own. You act as them, and see what they see and nothing more. An access token lasts %s; refresh it with the refresh token.\n\n", lasts(oauthAccessTTL))
	fmt.Fprintf(&b, "## Tools\n\n")
	for _, t := range tools {
		desc, _, _ := strings.Cut(t.Description, "\n")
		fmt.Fprintf(&b, "- `%s`: %s\n", t.Name, desc)
	}
	return b.String()
}

// lasts says a token's life as a person would: "an hour", "12 hours", "15 minutes".
func lasts(d time.Duration) string {
	switch {
	case d == time.Hour:
		return "an hour"
	case d%time.Hour == 0:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d minutes", int(d.Minutes()))
}
