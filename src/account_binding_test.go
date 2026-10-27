package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// A tab's cookies can change after it reads /auth/me. Even two editors of the
// same space must not accidentally submit each other's queued work.
func TestExpectedUserBindsRequestsToTheAuthenticatedAccount(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=wren@example.com")
	wren := decode[User](t, in.must("user", "add", "wren@example.com"))
	srv := in.serve()
	rook := decode[User](t, in.must("user", "add", "rook@example.com"))
	in.push(e2eSchema)
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "rook@example.com", "editor")

	login := func(email string) *http.Cookie {
		t.Helper()
		status, _ := call(t, "POST", srv.url+"/auth/magic", "", map[string]string{"email": email}, nil)
		wantStatus(t, status, 202, "request sign in")
		pattern := regexp.MustCompile(`sign-in link for ` + regexp.QuoteMeta(email) + `: (\S+)`)
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			links := pattern.FindAllStringSubmatch(srv.log.String(), -1)
			if len(links) == 0 {
				continue
			}
			response, err := http.Get(links[len(links)-1][1])
			if err != nil {
				t.Fatal("open sign-in link failed")
			}
			response.Body.Close()
			wantStatus(t, response.StatusCode, 200, "sign in")
			for _, cookie := range response.Cookies() {
				if cookie.Name == signinCookie && cookie.HttpOnly {
					return cookie
				}
			}
			t.Fatal("sign in did not set a session cookie")
		}
		t.Fatal("sign-in link was not delivered")
		return nil
	}
	wrenSession, rookSession := login(wren.Email), login(rook.Email)
	request := func(method, path string, cookie *http.Cookie, expected []string, body any) (int, http.Header) {
		t.Helper()
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, srv.url+path, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		for _, value := range expected {
			req.Header.Add("Archive-Expected-User", value)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal("account-bound request failed")
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		return response.StatusCode, response.Header
	}
	wrenID, rookID := []string{wren.ID.String()}, []string{rook.ID.String()}
	for _, check := range []struct {
		name     string
		cookie   *http.Cookie
		expected []string
		status   int
	}{
		{"legacy request", wrenSession, nil, 200},
		{"matching account", wrenSession, wrenID, 200},
		{"changed cookie", rookSession, wrenID, 412},
		{"empty precondition", rookSession, []string{""}, 400},
		{"invalid precondition", rookSession, []string{"not-a-uuid"}, 400},
		{"duplicate precondition", rookSession, []string{rook.ID.String(), rook.ID.String()}, 400},
		{"authentication first", nil, wrenID, 401},
	} {
		t.Run(check.name, func(t *testing.T) {
			status, _ := request("GET", "/auth/me", check.cookie, check.expected, nil)
			wantStatus(t, status, check.status, check.name)
		})
	}
	body := map[string]string{"code": `return await archive.create("person", {name: "Queued by Wren"}, {space: "family"});`}
	status, _ := request("POST", "/api/run", rookSession, wrenID, body)
	wantStatus(t, status, 412, "stale queued script")
	if got := in.must("list", "person"); strings.Contains(got, "Queued by Wren") {
		t.Fatal("rejected request executed its script")
	}
	status, _ = request("POST", "/api/person?space=family", rookSession, wrenID, map[string]string{"name": "Stale CRUD"})
	wantStatus(t, status, 412, "stale CRUD")
	for _, path := range []string{"/api/files/tus", "/api/files/tus/"} {
		status, _ = request("POST", path, rookSession, wrenID, nil)
		wantStatus(t, status, 412, "stale upload creation")
	}
	status, headers := request("POST", "/auth/signout", rookSession, wrenID, nil)
	wantStatus(t, status, 412, "stale signout")
	if len(headers.Values("Set-Cookie")) != 0 {
		t.Fatal("stale tab cleared the current cookie")
	}
	status, _ = request("GET", "/auth/me", rookSession, rookID, nil)
	wantStatus(t, status, 200, "current account remains signed in")
	status, _ = request("POST", "/api/person?space=family", rookSession, rookID, map[string]string{"name": "Submitted by Rook"})
	wantStatus(t, status, 201, "matching editor writes normally")
	if got := in.must("list", "person"); !strings.Contains(got, "Submitted by Rook") || strings.Contains(got, "Stale CRUD") {
		t.Fatal("account-bound writes did not preserve the expected rows")
	}
}
