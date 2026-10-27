package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
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

func mailPayload(headers map[string]string, body string, attachments []map[string]any, extra map[string]any) map[string]any {
	var text strings.Builder
	for _, h := range []string{"From", "To", "Cc", "Subject", "Date"} {
		if v := headers[h]; v != "" {
			text.WriteString(h + ": " + v + "\n")
		}
	}
	text.WriteString("\n" + body)
	if attachments == nil {
		attachments = []map[string]any{}
	}
	p := map[string]any{"headers": headers, "attachments": attachments, "text": text.String()}
	maps.Copy(p, extra)
	return p
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
			if err := r.file(ctx, att, name, b); err != nil {
				return err
			}
		}
		if err := r.emit(ctx, m.ID, mailPayload(headers, cmp.Or(body, m.Snippet), atts, map[string]any{
			"id": m.ID, "thread_id": m.ThreadID, "labels": m.LabelIDs, "internal_date": m.InternalDate})); err != nil {
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
	size, _ := strconv.Atoi(fmt.Sprint(f["size"]))
	var u string
	switch {
	case mt == "application/vnd.google-apps.document":
		u = base + "/files/" + url.PathEscape(id) + "/export?mimeType=text%2Fplain"
	case !strings.HasPrefix(mt, "application/vnd.google-apps.") && size <= syncMaxBody:
		u = base + "/files/" + url.PathEscape(id) + "?alt=media"
	}
	payload := map[string]any{"file": f, "text": ""}
	if u != "" {
		b, err := syncFetch(ctx, hc, u)
		switch {
		case errors.Is(err, ErrNotFound):
		case err != nil:
			return err
		case strings.HasPrefix(mt, "text/") || mt == "application/vnd.google-apps.document":
			payload["text"] = strings.ToValidUTF8(string(b), "�")
		default:
			name, _ := f["name"].(string)
			if err := r.file(ctx, payload, name, b); err != nil {
				return err
			}
		}
	}
	return r.emit(ctx, id, payload)
}

func (r *syncRun) file(ctx context.Context, into map[string]any, name string, b []byte) error {
	return r.a.as(ctx, func(ctx context.Context) error {
		f, err := r.a.fileAdd(ctx, r.src.Space, name, "", bytes.NewReader(b))
		if err == nil {
			into["file_id"] = f.ID.String()
		}
		return err
	})
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
	items, err := ScanFolder(ctx, cfg.Path, r.src.Space, st.Files)
	if err != nil {
		return err
	}
	for _, it := range items {
		r.res.Fetched++
		r.ingest(ctx, it)
	}
	return r.save(ctx, st)
}
