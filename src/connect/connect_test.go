package connect

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const gmailScope = "https://www.googleapis.com/auth/gmail.readonly"

func env(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }

// Every connector is declared whole, as an API connector or an MCP one, and Gmail asks for read-only mail
// with a refresh token on every consent.
func TestConnectorsAreDeclaredWhole(t *testing.T) {
	if len(All()) == 0 {
		t.Fatal("no connectors")
	}
	for _, c := range All() {
		switch {
		case !regexp.MustCompile(`^[a-z][a-z0-9]*$`).MatchString(c.ID) || c.Name == "":
			t.Errorf("connector %q is not named: %+v", c.ID, c)
		case c.MCP != "":
			if !strings.HasPrefix(c.MCP, "https://") || c.Env != "" || c.Host != "" || c.Endpoint.AuthURL != "" || c.Revoke != "" || len(c.Actions) > 0 {
				t.Errorf("MCP connector %q has what its server has: %+v", c.ID, c)
			}
		case !regexp.MustCompile(`^ARCHIVE_[A-Z0-9_]+$`).MatchString(c.Env) || !strings.HasPrefix(c.Host, "https://") ||
			c.Endpoint.AuthURL == "" || c.Endpoint.TokenURL == "" || c.Account == nil || len(c.Actions) == 0:
			t.Errorf("API connector %q is missing something: %+v", c.ID, c)
		}
		for name, a := range c.Actions {
			if !regexp.MustCompile(`^[a-z][a-z_]*$`).MatchString(name) || a.Doc == "" || a.Input == nil || a.Input.Type != "object" || a.Do == nil {
				t.Errorf("action %s.%s is not declared whole: %+v", c.ID, name, a)
			}
		}
	}
	g := registry["gmail"]
	if g == nil || !slices.Equal(g.Scopes, []string{gmailScope}) || g.Params["access_type"] != "offline" || g.Params["prompt"] != "consent" || g.Env != "ARCHIVE_GOOGLE" {
		t.Errorf("gmail: %+v", g)
	}
}

// Terraform makes a secret for every variable the service reads; an API connector's must be among them. An MCP
// connector needs none.
func TestTerraformHasEveryConnectorsVariable(t *testing.T) {
	b, err := os.ReadFile("../../apps/terraform/gcp/main.tf")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)\n  secrets = \{\n(.*?)\n  \}\n`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no local.secrets in apps/terraform/gcp/main.tf")
	}
	names := regexp.MustCompile(`(?m)^\s*([A-Z0-9_]+)\s*=`).FindAllSubmatch(m[1], -1)
	var listed []string
	for _, n := range names {
		listed = append(listed, string(n[1]))
	}
	for _, c := range All() {
		if c.MCP == "" && !slices.Contains(listed, c.Env) {
			t.Errorf("connector %s reads %s, which local.secrets in apps/terraform/gcp/main.tf does not list", c.ID, c.Env)
		}
	}
}

// A connector is available only with a whole client, and moves onto the host its client names.
func TestLoadReadsEachConnectorsClient(t *testing.T) {
	got, err := Load(env(nil))
	if err != nil || slices.ContainsFunc(All(), func(c *Connector) bool { return (got[c.ID] != nil) != (c.MCP != "") }) {
		t.Errorf("no clients set, and only the MCP connectors are available: %v %v", got, err)
	}
	if _, err := load(t, nil).Get("gmail"); err == nil || !strings.Contains(err.Error(), "Gmail is not set up on this server; it needs ARCHIVE_GOOGLE") {
		t.Errorf("an unset connector: %v", err)
	}
	if _, err := load(t, nil).Get("fax"); err == nil {
		t.Error("a connector there is none of")
	}
	if got, err := Load(env(map[string]string{"ARCHIVE_GOOGLE": "https://only-an-id@accounts.google.com"})); err != nil || got["gmail"] != nil {
		t.Errorf("a client without its secret: %v %v", got, err)
	}
	if _, err := Load(env(map[string]string{"ARCHIVE_GOOGLE": "accounts.google.com"})); err == nil || !strings.Contains(err.Error(), "ARCHIVE_GOOGLE: want https://<client id>:<client secret>@accounts.google.com") {
		t.Errorf("a client that is not a URL: %v", err)
	}
	g := registry["gmail"]
	p, err := load(t, map[string]string{"ARCHIVE_GOOGLE": "https://id.apps:s%2Fh@accounts.google.com"}).Get("gmail")
	if err != nil || p.clientID != "id.apps" || p.secret != "s/h" {
		t.Fatalf("Google's own host: %+v %v", p, err)
	}
	if cfg := p.config("https://api.example.com/cb"); cfg.Endpoint.AuthURL != g.Endpoint.AuthURL || cfg.Endpoint.TokenURL != g.Endpoint.TokenURL || p.at(g.Revoke) != g.Revoke {
		t.Errorf("on Google's own host the URLs stay Google's: %+v", cfg)
	}
	u := p.AuthURL("https://api.example.com/cb", "st", "v")
	for _, want := range []string{"access_type=offline", "prompt=consent", "code_challenge_method=S256", "state=st", "redirect_uri=https%3A%2F%2Fapi.example.com%2Fcb"} {
		if !strings.Contains(u, want) {
			t.Errorf("the consent URL %s has no %s", u, want)
		}
	}
	p, _ = load(t, map[string]string{"ARCHIVE_GOOGLE": "http://web:shh@127.0.0.1:9/fake"}).Get("gmail")
	if p.config("").Endpoint.TokenURL != "http://127.0.0.1:9/fake/token" || p.at("https://gmail.googleapis.com/gmail/v1/users/me/profile?x=1") != "http://127.0.0.1:9/fake/gmail/v1/users/me/profile?x=1" {
		t.Errorf("on a stand-in host every URL moves to it: %s", p.config("").Endpoint.TokenURL)
	}
}

func load(t *testing.T, kv map[string]string) Providers {
	t.Helper()
	ps, err := Load(env(kv))
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

// A state and a refresh token open only for what they were sealed for, and a state only until it expires.
func TestStatesAndTokensAreSealedForTheirPurpose(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(c)
	st := State{Provider: "gmail", Workspace: "w", User: uuid.New(), Member: uuid.New(), Verifier: "v", Exp: time.Now().Add(TTL).Unix()}
	sealed := SealState(aead, st)
	if back, ok := OpenState(aead, sealed); !ok || back != st || strings.Contains(sealed, "gmail") {
		t.Errorf("a sealed state comes back whole and unreadable: %+v %s", back, sealed)
	}
	if _, ok := OpenState(aead, sealed[:len(sealed)-2]+"AA"); ok {
		t.Error("a changed state opened")
	}
	st.Exp = time.Now().Add(-time.Second).Unix()
	if _, ok := OpenState(aead, SealState(aead, st)); ok {
		t.Error("an expired state opened")
	}
	id := uuid.New()
	box := SealToken(aead, id, "rt-1")
	if rt, err := OpenToken(aead, id, box); err != nil || rt != "rt-1" || strings.Contains(string(box), "rt-1") {
		t.Errorf("a sealed refresh token: %q %v", rt, err)
	}
	if _, err := OpenToken(aead, uuid.New(), box); err == nil {
		t.Error("a refresh token opened for another connection")
	}
	if _, ok := OpenState(aead, string(box)); ok {
		t.Error("a token opened as a state")
	}
}

// The web app draws every connector's logo, by its id.
func TestTheAppHasEveryConnectorsLogo(t *testing.T) {
	b, err := os.ReadFile("../../apps/web/src/lib/brands.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range All() {
		if !regexp.MustCompile(`(?m)^\s*` + c.ID + `: '`).Match(b) {
			t.Errorf("apps/web/src/lib/brands.js names no logo for connector %s", c.ID)
		}
	}
}

// The archive's connectors are integrations; a person's own mail, files and notes are theirs.
func TestConnectorsAreIntegrationsOrPersonal(t *testing.T) {
	var personal, integrations []string
	for _, c := range All() {
		if c.Integration {
			integrations = append(integrations, c.ID)
		} else {
			personal = append(personal, c.ID)
		}
	}
	if !slices.Equal(personal, []string{"gdrive", "gmail", "notion", "outlook"}) ||
		!slices.Equal(integrations, []string{"asana", "github", "gusto", "hubspot", "linear", "quickbooks", "sentry", "stripe"}) {
		t.Errorf("personal %v, integrations %v", personal, integrations)
	}
}
