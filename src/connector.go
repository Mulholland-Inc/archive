package main

import (
	"bufio"
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
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	_ "github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
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

type imapConfig struct {
	Host      string   `json:"host"`
	Username  string   `json:"username"`
	Mailboxes []string `json:"mailboxes"`
	StartTLS  bool     `json:"starttls"`
	Plaintext bool     `json:"plaintext"`
}

type imapCursor struct {
	UIDValidity uint32 `json:"uidvalidity"`
	UID         uint32 `json:"uid"`
}

const imapBatch = 25

func checkIMAP(_ context.Context, _ *Archive, src *Source, creds map[string]any) error {
	var cfg imapConfig
	if err := json.Unmarshal(src.Config, &cfg); err != nil {
		return sourceInvalid("config", err.Error())
	}
	switch pw, _ := creds["password"].(string); {
	case cfg.Host == "":
		return sourceInvalid("config.host", "want host:port, like imap.fastmail.com:993")
	case cfg.Username == "":
		return sourceInvalid("config.username", "required")
	case pw == "":
		return sourceInvalid("credentials.password", "required; use an app password for iCloud, Outlook and Fastmail")
	}
	return nil
}

func syncIMAP(ctx context.Context, r *syncRun) error {
	var cfg imapConfig
	json.Unmarshal(r.src.Config, &cfg)
	pw, _ := r.creds["password"].(string)
	dial := imapclient.DialTLS
	switch {
	case cfg.Plaintext:
		dial = imapclient.DialInsecure
	case cfg.StartTLS:
		dial = imapclient.DialStartTLS
	}
	c, err := dial(cfg.Host, nil)
	if err != nil {
		return fmt.Errorf("imap: %w", err)
	}
	defer c.Close()
	defer context.AfterFunc(ctx, func() { c.Close() })()
	if err := c.Login(cfg.Username, pw).Wait(); err != nil {
		return fmt.Errorf("imap login: %w", err)
	}
	var st struct {
		Mailboxes map[string]imapCursor `json:"mailboxes"`
	}
	json.Unmarshal(r.src.State, &st)
	if st.Mailboxes == nil {
		st.Mailboxes = map[string]imapCursor{}
	}
	if len(cfg.Mailboxes) == 0 {
		cfg.Mailboxes = []string{"INBOX"}
	}
	for _, mb := range cfg.Mailboxes {
		sel, err := c.Select(mb, &imap.SelectOptions{ReadOnly: true}).Wait()
		if err != nil {
			return fmt.Errorf("imap select %s: %w", mb, err)
		}
		cur := st.Mailboxes[mb]
		if cur.UIDValidity != sel.UIDValidity {
			cur = imapCursor{UIDValidity: sel.UIDValidity}
		}
		found, err := c.UIDSearch(&imap.SearchCriteria{UID: []imap.UIDSet{{{Start: imap.UID(cur.UID + 1), Stop: 0}}}}, nil).Wait()
		if err != nil {
			return fmt.Errorf("imap search %s: %w", mb, err)
		}
		uids := slices.DeleteFunc(found.AllUIDs(), func(u imap.UID) bool { return uint32(u) <= cur.UID })
		slices.Sort(uids)
		for chunk := range slices.Chunk(uids, imapBatch) {
			if err := r.limit.Wait(ctx); err != nil {
				return err
			}
			section := &imap.FetchItemBodySection{Peek: true}
			msgs, err := c.Fetch(imap.UIDSetNum(chunk...), &imap.FetchOptions{UID: true, BodySection: []*imap.FetchItemBodySection{section}}).Collect()
			if err != nil {
				return fmt.Errorf("imap fetch %s: %w", mb, err)
			}
			slices.SortFunc(msgs, func(x, y *imapclient.FetchMessageBuffer) int { return cmp.Compare(x.UID, y.UID) })
			for _, m := range msgs {
				headers, body, atts := imapParse(m.FindBodySection(section))
				id := fmt.Sprintf("%s:%d:%d", mb, cur.UIDValidity, m.UID)
				if err := r.emit(ctx, id, mailPayload(headers, body, atts, map[string]any{"mailbox": mb, "uid": m.UID})); err != nil {
					return err
				}
				cur.UID = max(cur.UID, uint32(m.UID))
			}
			st.Mailboxes[mb] = cur
			if err := r.save(ctx, st); err != nil {
				return err
			}
		}
		st.Mailboxes[mb] = cur
	}
	if err := r.save(ctx, st); err != nil {
		return err
	}
	return c.Logout().Wait()
}

func imapParse(b []byte) (map[string]string, string, []map[string]any) {
	headers := map[string]string{}
	mr, _ := gomail.CreateReader(bytes.NewReader(b))
	if mr == nil {
		return headers, strings.ToValidUTF8(string(b), "�"), nil
	}
	defer mr.Close()
	for _, h := range mailHeaders {
		if v, err := mr.Header.Text(h); err == nil && v != "" {
			headers[h] = v
		}
	}
	var body string
	var atts []map[string]any
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			if ct, _, _ := h.ContentType(); (ct == "text/plain" || ct == "") && body == "" {
				bb, _ := io.ReadAll(io.LimitReader(p.Body, syncMaxBody))
				body = strings.ToValidUTF8(string(bb), "�")
			}
		case *gomail.AttachmentHeader:
			name, _ := h.Filename()
			ct, _, _ := h.ContentType()
			n, _ := io.Copy(io.Discard, p.Body)
			atts = append(atts, map[string]any{"filename": name, "mime": ct, "size": n})
		}
	}
	return headers, body, atts
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
	var raws []*Raw
	err := r.a.as(ctx, func(ctx context.Context) (err error) {
		raws, err = ScanFolder(ctx, r.a.st, cfg.Path, r.src.Space)
		return err
	})
	for _, raw := range raws {
		r.res.Fetched++
		r.res.New++
		r.ingest(ctx, raw)
	}
	return err
}

type externalConfig struct {
	Command  []string       `json:"command"`
	Protocol string         `json:"protocol"`
	Streams  []string       `json:"streams"`
	Config   map[string]any `json:"config"`
}

type externalMsg struct {
	Type   string          `json:"type"`
	Record json.RawMessage `json:"record"`
	State  json.RawMessage `json:"state"`
	Value  json.RawMessage `json:"value"`
	Stream string          `json:"stream"`
	Keys   []string        `json:"key_properties"`
	Spec   *struct {
		ConnectionSpecification struct {
			Required []string `json:"required"`
		} `json:"connectionSpecification"`
	} `json:"spec"`
	ConnectionStatus *struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"connectionStatus"`
	Catalog *struct {
		Streams []json.RawMessage `json:"streams"`
	} `json:"catalog"`
	Trace *struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"trace"`
}

type externalStream struct {
	Name       string     `json:"name"`
	Modes      []string   `json:"supported_sync_modes"`
	Cursor     []string   `json:"default_cursor_field"`
	PrimaryKey [][]string `json:"source_defined_primary_key"`
}

type externalState struct {
	Airbyte map[string]json.RawMessage `json:"airbyte,omitempty"`
	Legacy  json.RawMessage            `json:"legacy,omitempty"`
	Singer  json.RawMessage            `json:"singer,omitempty"`
}

func externalSetup(src *Source, creds map[string]any) (*externalConfig, string, error) {
	var cfg externalConfig
	if err := json.Unmarshal(src.Config, &cfg); err != nil {
		return nil, "", sourceInvalid("config", err.Error())
	}
	if len(cfg.Command) == 0 {
		return nil, "", sourceInvalid("config.command", "want the connector executable and its arguments")
	}
	if cfg.Protocol != "" && cfg.Protocol != "airbyte" && cfg.Protocol != "singer" {
		return nil, "", sourceInvalid("config.protocol", "want airbyte or singer")
	}
	dir, err := os.MkdirTemp("", "archive-connector-")
	if err != nil {
		return nil, "", err
	}
	conf := map[string]any{}
	maps.Copy(conf, cfg.Config)
	maps.Copy(conf, creds)
	return &cfg, dir, externalWrite(dir, "config.json", conf)
}

func externalWrite(dir, name string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), b, 0o600)
}

func externalRun(ctx context.Context, cfg *externalConfig, dir string, args []string, fn func(*externalMsg) error) error {
	argv := slices.Clone(cfg.Command)
	for i := range argv {
		argv[i] = strings.ReplaceAll(argv[i], "{dir}", dir)
	}
	for i, a := range args {
		if strings.HasSuffix(a, ".json") {
			args[i] = filepath.Join(dir, a)
		}
	}
	cmd := exec.CommandContext(ctx, argv[0], append(argv[1:], args...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("connector: %w", err)
	}
	var ferr, trace error
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 0, 1<<20), syncMaxBody)
	for sc.Scan() {
		var m externalMsg
		if ferr != nil || json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		if m.Type == "TRACE" && m.Trace != nil && m.Trace.Error != nil {
			trace = errors.New(m.Trace.Error.Message)
		}
		ferr = fn(&m)
	}
	io.Copy(io.Discard, out)
	werr := cmd.Wait()
	if ferr != nil {
		return ferr
	}
	if werr != nil {
		tail := strings.TrimSpace(stderr.String())
		return fmt.Errorf("connector %s: %w", args[0], cmp.Or(trace, errors.New(cmp.Or(tail[max(0, len(tail)-500):], werr.Error()))))
	}
	return cmp.Or(sc.Err(), trace)
}

func checkExternal(ctx context.Context, _ *Archive, src *Source, creds map[string]any) error {
	cfg, dir, err := externalSetup(src, creds)
	if dir != "" {
		defer os.RemoveAll(dir)
	}
	if err != nil || cfg.Protocol == "singer" {
		return err
	}
	conf := maps.Clone(cfg.Config)
	if conf == nil {
		conf = map[string]any{}
	}
	maps.Copy(conf, creds)
	verr := &ValidationError{Type: "source"}
	err = externalRun(ctx, cfg, dir, []string{"spec"}, func(m *externalMsg) error {
		if m.Type == "SPEC" && m.Spec != nil {
			for _, k := range m.Spec.ConnectionSpecification.Required {
				if conf[k] == nil {
					verr.Errors = append(verr.Errors, FieldError{Path: "config." + k, Message: "required by the connector"})
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(verr.Errors) > 0 {
		return verr
	}
	return externalRun(ctx, cfg, dir, []string{"check", "--config", "config.json"}, func(m *externalMsg) error {
		if m.Type == "CONNECTION_STATUS" && m.ConnectionStatus != nil && m.ConnectionStatus.Status != "SUCCEEDED" {
			return sourceInvalid("config", "connector check failed: "+m.ConnectionStatus.Message)
		}
		return nil
	})
}

func syncExternal(ctx context.Context, r *syncRun) error {
	cfg, dir, err := externalSetup(r.src, r.creds)
	if dir != "" {
		defer os.RemoveAll(dir)
	}
	if err != nil {
		return err
	}
	var st externalState
	json.Unmarshal(r.src.State, &st)
	keys := map[string][][]string{}
	record := func(stream string, data json.RawMessage) error {
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("connector record: %w", err)
		}
		return r.emit(ctx, externalID(stream, keys[stream], m, data), map[string]any{"stream": stream, "data": m, "text": string(data)})
	}
	if cfg.Protocol == "singer" {
		args := []string{"--config", "config.json"}
		if st.Singer != nil {
			if err := os.WriteFile(filepath.Join(dir, "state.json"), st.Singer, 0o600); err != nil {
				return err
			}
			args = append(args, "--state", "state.json")
		}
		return externalRun(ctx, cfg, dir, args, func(m *externalMsg) error {
			switch m.Type {
			case "SCHEMA":
				keys[m.Stream] = nil
				for _, k := range m.Keys {
					keys[m.Stream] = append(keys[m.Stream], []string{k})
				}
			case "RECORD":
				return record(m.Stream, m.Record)
			case "STATE":
				st.Singer = m.Value
				return r.save(ctx, st)
			}
			return nil
		})
	}
	var streams []map[string]any
	err = externalRun(ctx, cfg, dir, []string{"discover", "--config", "config.json"}, func(m *externalMsg) error {
		if m.Type != "CATALOG" || m.Catalog == nil {
			return nil
		}
		for _, raw := range m.Catalog.Streams {
			var s externalStream
			if json.Unmarshal(raw, &s) != nil || len(cfg.Streams) > 0 && !slices.Contains(cfg.Streams, s.Name) {
				continue
			}
			mode := "full_refresh"
			if slices.Contains(s.Modes, "incremental") {
				mode = "incremental"
			}
			keys[s.Name] = s.PrimaryKey
			c := map[string]any{"stream": raw, "sync_mode": mode, "destination_sync_mode": "append"}
			if s.Cursor != nil {
				c["cursor_field"] = s.Cursor
			}
			if s.PrimaryKey != nil {
				c["primary_key"] = s.PrimaryKey
			}
			streams = append(streams, c)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := externalWrite(dir, "catalog.json", map[string]any{"streams": streams}); err != nil {
		return err
	}
	args := []string{"read", "--config", "config.json", "--catalog", "catalog.json"}
	if st.Legacy != nil || len(st.Airbyte) > 0 {
		var state any = st.Legacy
		if st.Legacy == nil {
			state = slices.Collect(maps.Values(st.Airbyte))
		}
		if err := externalWrite(dir, "state.json", state); err != nil {
			return err
		}
		args = append(args, "--state", "state.json")
	}
	return externalRun(ctx, cfg, dir, args, func(m *externalMsg) error {
		switch m.Type {
		case "RECORD":
			var rec struct {
				Stream string          `json:"stream"`
				Data   json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(m.Record, &rec); err != nil {
				return fmt.Errorf("connector record: %w", err)
			}
			return record(rec.Stream, rec.Data)
		case "STATE":
			var s struct {
				Type   string          `json:"type"`
				Data   json.RawMessage `json:"data"`
				Stream struct {
					Descriptor struct {
						Name      string `json:"name"`
						Namespace string `json:"namespace"`
					} `json:"stream_descriptor"`
				} `json:"stream"`
			}
			if err := json.Unmarshal(m.State, &s); err != nil {
				return fmt.Errorf("connector state: %w", err)
			}
			switch s.Type {
			case "STREAM":
				if st.Airbyte == nil {
					st.Airbyte = map[string]json.RawMessage{}
				}
				st.Airbyte[s.Stream.Descriptor.Namespace+"."+s.Stream.Descriptor.Name] = m.State
			case "GLOBAL":
				st.Airbyte = map[string]json.RawMessage{"": m.State}
			default:
				st.Legacy = s.Data
			}
			return r.save(ctx, st)
		}
		return nil
	})
}

func externalID(stream string, pk [][]string, m map[string]any, data []byte) string {
	var parts []string
	for _, path := range pk {
		var v any = m
		for _, k := range path {
			obj, _ := v.(map[string]any)
			v = obj[k]
		}
		if v == nil {
			parts = nil
			break
		}
		parts = append(parts, fmt.Sprint(v))
	}
	if len(parts) == 0 {
		sum := sha256.Sum256(data)
		return stream + ":" + hex.EncodeToString(sum[:12])
	}
	return stream + ":" + strings.Join(parts, ",")
}
