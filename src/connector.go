package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	_ "github.com/emersion/go-message/charset"
	"golang.org/x/oauth2"
)

const syncMaxBody = 32 << 20

var mailHeaders = []string{"From", "To", "Cc", "Subject", "Date", "Message-ID", "In-Reply-To", "References", "Reply-To"}

func mailText(headers map[string]string, body string) string {
	var text strings.Builder
	for _, h := range []string{"From", "To", "Cc", "Subject", "Date"} {
		if v := headers[h]; v != "" {
			text.WriteString(h + ": " + v + "\n")
		}
	}
	return text.String() + "\n" + body
}

func mailEML(headers map[string]string, body string) []byte {
	var b strings.Builder
	for _, h := range mailHeaders {
		if v := headers[h]; v != "" {
			b.WriteString(h + ": " + strings.ReplaceAll(strings.ReplaceAll(v, "\r", ""), "\n", " ") + "\r\n")
		}
	}
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + body)
	return []byte(b.String())
}

func googleAPI() string {
	return strings.TrimRight(coreEnv("ARCHIVE_GOOGLE_API_URL", "https://www.googleapis.com"), "/")
}

func checkGoogle(_ context.Context, _ *Archive, _ *Source, _ map[string]any) error {
	_, err := googleOAuth("", nil)
	return err
}

func (r *syncRun) google(ctx context.Context) (*http.Client, error) {
	cfg, err := googleOAuth("", googleScopes[r.src.Kind])
	if err != nil {
		return nil, err
	}
	rt, _ := r.creds["refresh_token"].(string)
	return cfg.Client(context.WithValue(ctx, oauth2.HTTPClient, r.http), &oauth2.Token{RefreshToken: rt}), nil
}

func syncFetch(ctx context.Context, hc *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, syncMaxBody))
	path, _, _ := strings.Cut(u, "?")
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("GET %s: %w", path, ErrNotFound)
	case resp.StatusCode/100 != 2:
		return nil, fmt.Errorf("GET %s: %s: %s", path, resp.Status, bytes.TrimSpace(b[:min(len(b), 300)]))
	}
	return b, err
}

func syncGet(ctx context.Context, hc *http.Client, u string, out any) error {
	b, err := syncFetch(ctx, hc, u)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

type gmailPart struct {
	MimeType string `json:"mimeType"`
	Filename string `json:"filename"`
	Headers  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	Body struct {
		AttachmentID string `json:"attachmentId"`
		Size         int    `json:"size"`
		Data         string `json:"data"`
	} `json:"body"`
	Parts []gmailPart `json:"parts"`
}

type gmailMessage struct {
	ID           string    `json:"id"`
	ThreadID     string    `json:"threadId"`
	LabelIDs     []string  `json:"labelIds"`
	Snippet      string    `json:"snippet"`
	InternalDate string    `json:"internalDate"`
	Payload      gmailPart `json:"payload"`
}

func (p *gmailPart) walk(body *string, atts *[]map[string]any) {
	if p.Filename != "" {
		*atts = append(*atts, map[string]any{"filename": p.Filename, "mime": p.MimeType, "size": p.Body.Size, "attachment_id": p.Body.AttachmentID})
	} else if p.MimeType == "text/plain" && *body == "" {
		if b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(p.Body.Data, "=")); err == nil {
			*body = string(b)
		}
	}
	for i := range p.Parts {
		p.Parts[i].walk(body, atts)
	}
}

func syncGmail(ctx context.Context, r *syncRun) error {
	hc, err := r.google(ctx)
	if err != nil {
		return err
	}
	base := googleAPI() + "/gmail/v1/users/me"
	var st struct {
		HistoryID string `json:"history_id"`
	}
	json.Unmarshal(r.src.State, &st)
	var ids []string
	next := st.HistoryID
	if st.HistoryID != "" {
		ids, next, err = gmailHistory(ctx, hc, base, st.HistoryID)
		if errors.Is(err, ErrNotFound) {
			st.HistoryID = ""
		} else if err != nil {
			return err
		}
	}
	if st.HistoryID == "" {
		var p struct {
			HistoryID string `json:"historyId"`
		}
		if err := syncGet(ctx, hc, base+"/profile", &p); err != nil {
			return err
		}
		var cfg struct {
			Query string `json:"query"`
		}
		json.Unmarshal(r.src.Config, &cfg)
		if ids, err = gmailList(ctx, hc, base, cmp.Or(cfg.Query, "newer_than:30d")); err != nil {
			return err
		}
		next = p.HistoryID
	}
	for _, id := range ids {
		var m gmailMessage
		if err := syncGet(ctx, hc, base+"/messages/"+url.PathEscape(id)+"?format=full", &m); errors.Is(err, ErrNotFound) {
			continue
		} else if err != nil {
			return err
		}
		headers := map[string]string{}
		for _, h := range m.Payload.Headers {
			for _, want := range mailHeaders {
				if strings.EqualFold(h.Name, want) && headers[want] == "" {
					headers[want] = h.Value
				}
			}
		}
		var body string
		var atts []map[string]any
		m.Payload.walk(&body, &atts)
		for _, att := range atts {
			aid, _ := att["attachment_id"].(string)
			if aid == "" {
				continue
			}
			var data struct {
				Data string `json:"data"`
			}
			if err := syncGet(ctx, hc, base+"/messages/"+url.PathEscape(m.ID)+"/attachments/"+url.PathEscape(aid), &data); err != nil {
				return err
			}
			b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(data.Data, "="))
			if err != nil {
				return fmt.Errorf("gmail attachment %s: %w", aid, err)
			}
			name, _ := att["filename"].(string)
			if _, err := r.file(ctx, name, b); err != nil {
				return err
			}
		}
		if _, err := r.file(ctx, cmp.Or(headers["Subject"], m.ID)+".eml", mailEML(headers, cmp.Or(body, m.Snippet))); err != nil {
			return err
		}
	}
	return r.save(ctx, map[string]string{"history_id": next})
}

func gmailList(ctx context.Context, hc *http.Client, base, query string) ([]string, error) {
	var ids []string
	for page := ""; ; {
		var out struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			NextPageToken string `json:"nextPageToken"`
		}
		q := url.Values{"q": {query}, "maxResults": {"100"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		if err := syncGet(ctx, hc, base+"/messages?"+q.Encode(), &out); err != nil {
			return nil, err
		}
		for _, m := range out.Messages {
			ids = append(ids, m.ID)
		}
		if page = out.NextPageToken; page == "" {
			slices.Reverse(ids)
			return ids, nil
		}
	}
}

func gmailHistory(ctx context.Context, hc *http.Client, base, start string) ([]string, string, error) {
	var ids []string
	seen := map[string]bool{}
	for page := ""; ; {
		var out struct {
			History []struct {
				MessagesAdded []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"messagesAdded"`
			} `json:"history"`
			HistoryID     string `json:"historyId"`
			NextPageToken string `json:"nextPageToken"`
		}
		q := url.Values{"startHistoryId": {start}, "historyTypes": {"messageAdded"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		if err := syncGet(ctx, hc, base+"/history?"+q.Encode(), &out); err != nil {
			return nil, "", err
		}
		for _, h := range out.History {
			for _, m := range h.MessagesAdded {
				if !seen[m.Message.ID] {
					seen[m.Message.ID] = true
					ids = append(ids, m.Message.ID)
				}
			}
		}
		if page = out.NextPageToken; page == "" {
			return ids, cmp.Or(out.HistoryID, start), nil
		}
	}
}

const driveFields = "id,name,mimeType,modifiedTime,createdTime,size,md5Checksum,parents,webViewLink,trashed,owners(emailAddress,displayName)"

func syncDrive(ctx context.Context, r *syncRun) error {
	hc, err := r.google(ctx)
	if err != nil {
		return err
	}
	base := googleAPI() + "/drive/v3"
	var st struct {
		PageToken string `json:"page_token"`
	}
	json.Unmarshal(r.src.State, &st)
	if st.PageToken == "" {
		var start struct {
			StartPageToken string `json:"startPageToken"`
		}
		if err := syncGet(ctx, hc, base+"/changes/startPageToken", &start); err != nil {
			return err
		}
		for page := ""; ; {
			var out struct {
				Files         []map[string]any `json:"files"`
				NextPageToken string           `json:"nextPageToken"`
			}
			q := url.Values{"q": {"trashed = false"}, "pageSize": {"100"}, "fields": {"nextPageToken,files(" + driveFields + ")"}}
			if page != "" {
				q.Set("pageToken", page)
			}
			if err := syncGet(ctx, hc, base+"/files?"+q.Encode(), &out); err != nil {
				return err
			}
			for _, f := range out.Files {
				if err := r.driveFile(ctx, hc, base, f); err != nil {
					return err
				}
			}
			if page = out.NextPageToken; page == "" {
				break
			}
		}
		return r.save(ctx, map[string]string{"page_token": start.StartPageToken})
	}
	for tok := st.PageToken; ; {
		var out struct {
			Changes []struct {
				Removed bool           `json:"removed"`
				File    map[string]any `json:"file"`
			} `json:"changes"`
			NextPageToken     string `json:"nextPageToken"`
			NewStartPageToken string `json:"newStartPageToken"`
		}
		q := url.Values{"pageToken": {tok}, "pageSize": {"100"}, "fields": {"nextPageToken,newStartPageToken,changes(removed,file(" + driveFields + "))"}}
		if err := syncGet(ctx, hc, base+"/changes?"+q.Encode(), &out); err != nil {
			return err
		}
		for _, c := range out.Changes {
			if c.Removed || c.File == nil || c.File["trashed"] == true {
				continue
			}
			if err := r.driveFile(ctx, hc, base, c.File); err != nil {
				return err
			}
		}
		tok = cmp.Or(out.NextPageToken, out.NewStartPageToken)
		if err := r.save(ctx, map[string]string{"page_token": tok}); err != nil || out.NextPageToken == "" {
			return err
		}
	}
}

func (r *syncRun) driveFile(ctx context.Context, hc *http.Client, base string, f map[string]any) error {
	id, _ := f["id"].(string)
	mt, _ := f["mimeType"].(string)
	name, _ := f["name"].(string)
	size, _ := strconv.Atoi(fmt.Sprint(f["size"]))
	var u string
	switch {
	case mt == "application/vnd.google-apps.document":
		u, name = base+"/files/"+url.PathEscape(id)+"/export?mimeType=text%2Fplain", name+".txt"
	case !strings.HasPrefix(mt, "application/vnd.google-apps.") && size <= syncMaxBody:
		u = base + "/files/" + url.PathEscape(id) + "?alt=media"
	default:
		return nil
	}
	b, err := syncFetch(ctx, hc, u)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err == nil {
		_, err = r.file(ctx, name, b)
	}
	return err
}

func checkFolder(_ context.Context, _ *Archive, src *Source, _ map[string]any) error {
	var cfg map[string]any
	json.Unmarshal(src.Config, &cfg)
	p, _ := cfg["path"].(string)
	abs, err := filepath.Abs(p)
	if fi, serr := os.Stat(abs); p == "" || err != nil || serr != nil || !fi.IsDir() {
		return sourceInvalid("config.path", "want a folder on the Archive server")
	}
	cfg["path"] = abs
	src.Config, err = json.Marshal(cfg)
	return err
}

func syncFolder(ctx context.Context, r *syncRun) error {
	var cfg struct {
		Path string `json:"path"`
	}
	json.Unmarshal(r.src.Config, &cfg)
	var st struct {
		Files map[string]string `json:"files"`
	}
	json.Unmarshal(r.src.State, &st)
	if st.Files == nil {
		st.Files = map[string]string{}
	}
	err := filepath.WalkDir(cfg.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != cfg.Path && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(cfg.Path, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		if hash := hex.EncodeToString(sum[:]); st.Files[filepath.ToSlash(rel)] != hash {
			if _, err := r.file(ctx, d.Name(), b); err != nil {
				return err
			}
			st.Files[filepath.ToSlash(rel)] = hash
		}
		return nil
	})
	if err != nil {
		return err
	}
	return r.save(ctx, st)
}
