package archive

import (
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// e2eSignIn signs email in through a magic link from the log and returns the session cookie.
func e2eSignIn(t *testing.T, srv *e2eServer, email string) *http.Cookie {
	t.Helper()
	call(t, "POST", srv.url+"/auth/magic", "", map[string]any{"email": email}, nil)
	re := regexp.MustCompile(`sign-in link for ` + regexp.QuoteMeta(email) + `: (\S+)`)
	var link string
	for deadline := time.Now().Add(5 * time.Second); link == "" && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if m := re.FindAllStringSubmatch(srv.log.String(), -1); m != nil {
			link = m[len(m)-1][1]
		}
	}
	resp, err := http.Get(link)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == "archive_session" {
			return c
		}
	}
	t.Fatalf("no session for %s", email)
	return nil
}

// A space admin invites someone who has no account yet by email. The link shows who invited them
// and to what, and only a press of its button uses it, so a mail scanner's GET does not. It signs
// them in once, makes their user, and grants the level chosen in that space and nowhere else.
func TestASpaceAdminInvitesSomeoneNewWithOneLinkThatWorksOnce(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("space", "add", "family")
	in.must("space", "add", "work")
	in.must("space", "grant", "family", "bob@example.com", "admin")
	bob := e2eSignIn(t, srv, "bob@example.com")

	var inv struct {
		Link      string    `json:"link"`
		Role      string    `json:"role"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	st, _ := call(t, "POST", srv.url+"/auth/spaces/family/invites", "", map[string]any{"email": "Dana@Example.com", "role": "viewer"}, &inv, bob)
	wantStatus(t, st, 201, "a space admin invites")
	if !strings.Contains(inv.Link, "/auth/invite?token=") || inv.Role != "viewer" || time.Until(inv.ExpiresAt) < 6*24*time.Hour {
		t.Fatalf("invite: %+v", inv)
	}
	// the log comes through a pipe, so the line can land a moment after the response
	for deadline := time.Now().Add(5 * time.Second); !strings.Contains(srv.log.String(), "invite link for dana@example.com"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no invite mail (logged without SMTP):\n%s", srv.log)
		}
	}

	t.Run("NoHigherThanTheInviterAndOnlyAsAnAdmin", func(t *testing.T) {
		st, _ := call(t, "POST", srv.url+"/auth/spaces/family/invites", "", map[string]any{"email": "eve@example.com", "role": "owner"}, nil, bob)
		wantStatus(t, st, 403, "an admin invites an owner")
		st, _ = call(t, "POST", srv.url+"/auth/spaces/work/invites", "", map[string]any{"email": "eve@example.com", "role": "viewer"}, nil, bob)
		if st != 403 && st != 404 {
			t.Errorf("bob invites to a space he does not admin: %d", st)
		}
		// as with membership, only a person or a full token, as Terraform uses, manages who is in
		tok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create", "-space", "family:admin")).Secret
		st, _ = call(t, "POST", srv.url+"/auth/spaces/family/invites", tok, map[string]any{"email": "eve@example.com", "role": "viewer"}, nil)
		wantStatus(t, st, 403, "a space-scoped token invites")
	})

	t.Run("OpeningTheLinkDoesNotUseIt", func(t *testing.T) {
		req, _ := http.NewRequest("GET", inv.Link, nil)
		req.Header.Set("Accept", "text/html")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(string(b), "Bob") || !strings.Contains(string(b), "family") || !strings.Contains(string(b), "<form") {
			t.Fatalf("invite page: %d %s", resp.StatusCode, b)
		}
		if strings.Contains(string(b), "<script") {
			t.Errorf("the invite page runs script")
		}
	})

	var dana *http.Cookie
	t.Run("PressingJoinSignsThemInWithTheLevel", func(t *testing.T) {
		u, _ := url.Parse(inv.Link)
		c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		req, _ := http.NewRequest("POST", srv.url+"/auth/invite", strings.NewReader(url.Values{"token": {u.Query().Get("token")}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "text/html")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
			t.Fatalf("join: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}
		for _, k := range resp.Cookies() {
			if k.Name == "archive_session" {
				dana = k
			}
		}
		if dana == nil {
			t.Fatal("no session")
		}
		var sp []SpaceInfo
		call(t, "GET", srv.url+"/auth/spaces", "", nil, &sp, dana)
		levels := map[string]string{}
		for _, s := range sp {
			levels[s.Name] = s.Level
		}
		if levels["family"] != "viewer" || levels["work"] != "" {
			t.Errorf("dana's spaces: %v", levels)
		}
	})

	t.Run("TheLinkWorksOnce", func(t *testing.T) {
		u, _ := url.Parse(inv.Link)
		st, _ := call(t, "POST", srv.url+"/auth/invite", "", map[string]any{"token": u.Query().Get("token")}, nil)
		wantStatus(t, st, 401, "the link again")
	})

	t.Run("AnExistingUserIsGrantedNotRemade", func(t *testing.T) {
		in := in.on(t)
		in.must("user", "add", "carol@example.com")
		var again struct {
			Link string `json:"link"`
		}
		call(t, "POST", srv.url+"/auth/spaces/family/invites", "", map[string]any{"email": "carol@example.com", "role": "editor"}, &again, bob)
		u, _ := url.Parse(again.Link)
		st, _ := call(t, "POST", srv.url+"/auth/invite", "", map[string]any{"token": u.Query().Get("token")}, nil)
		wantStatus(t, st, 200, "carol joins")
		if n := in.psql("archive", "SELECT count(*) FROM users WHERE email = 'carol@example.com'"); n != "1" {
			t.Errorf("%s carols", n)
		}
	})
}

// A product invites someone by the name whoever invites them gives: the account the invite makes
// takes it, and so does an account that has no name yet, but a name someone already has is theirs.
func TestAnInviteCarriesTheInviteesNameToTheirAccount(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "toymodule")
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/toymodule").CombinedOutput(); err != nil {
		t.Fatalf("build the toy module: %v\n%s", err, out)
	}
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	in.bin = bin
	srv := in.serve()
	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("user", "add", "erin@example.com")
	in.must("user", "add", "frank@example.com", "-name", "Frank")
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "admin")
	bob := e2eSignIn(t, srv, "bob@example.com")

	for email, name := range map[string]string{"dana@example.com": "  Dana Quist ", "erin@example.com": "Erin Quist", "frank@example.com": "Francis Quist"} {
		var got struct {
			Invite Invite `json:"invite"`
			Token  string `json:"token"`
		}
		st, _ := call(t, "POST", srv.url+"/api/toy/invites", "", map[string]any{"space": "family", "email": email, "name": name, "role": "viewer"}, &got, bob)
		wantStatus(t, st, 201, "bob invites "+email)
		if got.Invite.Name != strings.TrimSpace(name) || got.Token == "" {
			t.Fatalf("invite: %+v", got.Invite)
		}
		resp, _ := e2eGet(t, srv.url+"/auth/invite?token="+url.QueryEscape(got.Token), "")
		wantStatus(t, resp.StatusCode, 200, "the invite opens")
		st, _ = call(t, "POST", srv.url+"/auth/invite", "", map[string]any{"token": got.Token}, nil)
		wantStatus(t, st, 200, email+" joins")
	}
	for email, want := range map[string]string{"dana@example.com": "Dana Quist", "erin@example.com": "Erin Quist", "frank@example.com": "Frank"} {
		if got := in.psql("archive", "SELECT name FROM users WHERE email = '"+email+"'"); got != want {
			t.Errorf("%s is named %q, want %q", email, got, want)
		}
	}

	t.Run("AnInviteWithoutANameLeavesItEmpty", func(t *testing.T) {
		in := in.on(t)
		var inv struct {
			Link string `json:"link"`
		}
		call(t, "POST", srv.url+"/auth/spaces/family/invites", "", map[string]any{"email": "gil@example.com", "role": "viewer"}, &inv, bob)
		u, _ := url.Parse(inv.Link)
		st, _ := call(t, "POST", srv.url+"/auth/invite", "", map[string]any{"token": u.Query().Get("token")}, nil)
		wantStatus(t, st, 200, "gil joins")
		if got := in.psql("archive", "SELECT name FROM users WHERE email = 'gil@example.com'"); got != "" {
			t.Errorf("gil is named %q", got)
		}
	})
}
