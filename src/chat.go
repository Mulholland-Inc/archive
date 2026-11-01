package main

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	chatAPIVersion = "2026-10-21"
	chatMaxTokens  = 64000
	chatMaxRuns    = 25
	chatTitleMax   = 60
)

type Chat struct {
	ID        uuid.UUID       `json:"id"`
	Title     string          `json:"title"`
	Messages  json.RawMessage `json:"messages,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

const chatModel = "claude-haiku-5-5"

func chatUser(ctx context.Context) (uuid.UUID, error) {
	ac := ActorFrom(ctx)
	if ac == nil {
		return uuid.Nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	return ac.UserID, nil
}

func chatFound(id uuid.UUID, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("chat %s: %w", id, ErrNotFound)
	}
	return err
}

func (a *Archive) Chats(ctx context.Context) ([]Chat, error) {
	uid, err := chatUser(ctx)
	if err != nil {
		return nil, err
	}
	out := []Chat{}
	return out, a.sys(ctx, func(ctx context.Context) error {
		rows, err := a.st.db(ctx).Chats(ctx, uid)
		for _, r := range rows {
			out = append(out, Chat{ID: r.ID, Title: r.Title, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
		}
		return err
	})
}

func (a *Archive) Chat(ctx context.Context, id uuid.UUID) (*Chat, error) {
	uid, err := chatUser(ctx)
	if err != nil {
		return nil, err
	}
	var c generated.ArchiveChat
	err = a.sys(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).ChatByID(ctx, id, uid)
		return err
	})
	if err != nil {
		return nil, chatFound(id, err)
	}
	return &Chat{ID: c.ID, Title: c.Title, Messages: c.Messages, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}, nil
}

func (a *Archive) RenameChat(ctx context.Context, id uuid.UUID, title string) (*Chat, error) {
	uid, err := chatUser(ctx)
	if err != nil {
		return nil, err
	}
	title = chatTitle(title)
	if title == "" {
		return nil, fmt.Errorf("title: want a name: %w", httpErrBadRequest)
	}
	var n int64
	if err := a.sys(ctx, func(ctx context.Context) (err error) {
		n, err = a.st.db(ctx).RenameChat(ctx, id, uid, title)
		return err
	}); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, chatFound(id, pgx.ErrNoRows)
	}
	c, err := a.Chat(ctx, id)
	if c != nil {
		c.Messages = nil
	}
	return c, err
}

func (a *Archive) DeleteChat(ctx context.Context, id uuid.UUID) error {
	uid, err := chatUser(ctx)
	if err != nil {
		return err
	}
	var n int64
	if err := a.sys(ctx, func(ctx context.Context) (err error) {
		n, err = a.st.db(ctx).DeleteChat(ctx, id, uid)
		return err
	}); err != nil {
		return err
	}
	if n == 0 {
		return chatFound(id, pgx.ErrNoRows)
	}
	return nil
}

func chatTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= chatTitleMax {
		return s
	}
	r := []rune(s)[:chatTitleMax]
	if i := strings.LastIndexByte(string(r), ' '); i > chatTitleMax/2 {
		return string(r)[:i] + "…"
	}
	return string(r) + "…"
}

func (a *Archive) chatAppend(ctx context.Context, id uuid.UUID, msgs ...any) error {
	b, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	return a.sys(context.WithoutCancel(ctx), func(ctx context.Context) error {
		return a.st.db(ctx).AppendChat(ctx, b, id)
	})
}

type chatBlock = map[string]any

type chatMessage struct {
	Role    string      `json:"role"`
	Content []chatBlock `json:"content"`
}

func chatUses(m chatMessage) []chatBlock {
	var out []chatBlock
	for _, b := range m.Content {
		if b["type"] == "tool_use" {
			out = append(out, b)
		}
	}
	return out
}

func chatToolError(uses []chatBlock, msg string) *chatMessage {
	if len(uses) == 0 {
		return nil
	}
	m := &chatMessage{Role: "user"}
	for _, u := range uses {
		m.Content = append(m.Content, chatBlock{"type": "tool_result", "tool_use_id": u["id"], "content": msg, "is_error": true})
	}
	return m
}

func (a *Archive) chat(w http.ResponseWriter, r *http.Request) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		httpWrite(w, http.StatusServiceUnavailable, map[string]string{"error": "the agent needs ANTHROPIC_API_KEY set on the server"}, nil)
		return
	}
	var body struct {
		Chat uuid.UUID `json:"chat"`
		Text string    `json:"text"`
	}
	if err := httpDecode(r, &body); err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	ctx := r.Context()
	ac := ActorFrom(ctx)
	text := strings.TrimSpace(body.Text)
	var c *Chat
	var err error
	if body.Chat == uuid.Nil {
		if text == "" {
			httpWrite(w, 0, nil, fmt.Errorf("text: want a message: %w", httpErrBadRequest))
			return
		}
		err = a.sys(ctx, func(ctx context.Context) error {
			row, err := a.st.db(ctx).CreateChat(ctx, ac.UserID, chatTitle(text))
			c = &Chat{ID: row.ID, Title: row.Title, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Messages: row.Messages}
			return err
		})
	} else {
		c, err = a.Chat(ctx, body.Chat)
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	var msgs []chatMessage
	if err := json.Unmarshal(c.Messages, &msgs); err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	var fresh []any
	if len(msgs) > 0 && msgs[len(msgs)-1].Role == "assistant" {
		if m := chatToolError(chatUses(msgs[len(msgs)-1]), "stopped before this ran"); m != nil {
			msgs, fresh = append(msgs, *m), append(fresh, m)
		}
	}
	if text != "" {
		m := chatMessage{Role: "user", Content: []chatBlock{{"type": "text", "text": text}}}
		msgs, fresh = append(msgs, m), append(fresh, m)
	} else if len(msgs) == 0 || msgs[len(msgs)-1].Role != "user" {
		httpWrite(w, 0, nil, fmt.Errorf("text: want a message, the chat has nothing to answer: %w", httpErrBadRequest))
		return
	}
	if len(fresh) > 0 {
		if err := a.chatAppend(ctx, c.ID, fresh...); err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	rc := http.NewResponseController(w)
	send := func(v map[string]any) {
		if ctx.Err() == nil {
			enc.Encode(v)
			rc.Flush()
		}
	}
	send(map[string]any{"type": "chat", "chat": Chat{ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}})
	fail := func(err error) { send(map[string]any{"type": "error", "error": err.Error()}) }
	tools := []any{map[string]any{
		"name":        "run",
		"description": a.RunDocs(ctx),
		"input_schema": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"code": map[string]any{"type": "string", "description": "TypeScript to run inside Archive against the archive SDK in this tool's description"}},
			"required":             []string{"code"},
			"additionalProperties": false,
		},
		"eager_input_streaming": true,
		"cache_control":         map[string]string{"type": "ephemeral"},
	}}
	system := chatSystem(ac)
	for runs := 0; ; {
		msg, stop, err := chatTurn(ctx, key, tools, system, msgs, send)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			fail(err)
			return
		}
		msgs = append(msgs, *msg)
		if err := a.chatAppend(ctx, c.ID, msg); err != nil {
			fail(err)
			return
		}
		uses := chatUses(*msg)
		if stop != "tool_use" || len(uses) == 0 {
			if m := chatToolError(uses, "the answer was cut off before this call was complete"); m != nil {
				msgs = append(msgs, *m)
				a.chatAppend(ctx, c.ID, m)
			}
			switch stop {
			case "refusal":
				fail(errors.New("the model declined to answer this"))
			case "max_tokens":
				fail(errors.New("the answer hit the length limit"))
			default:
				send(map[string]any{"type": "done", "stop": stop})
			}
			return
		}
		res := chatMessage{Role: "user"}
		for _, u := range uses {
			id, _ := u["id"].(string)
			code, ok := chatCode(u["input"])
			out := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}
			switch {
			case !ok:
				out.Error = "the input was not an object with a code string; call run again"
			case runs >= chatMaxRuns:
				out.Error = fmt.Sprintf("this answer already ran code %d times, the limit; answer with what you have", chatMaxRuns)
			default:
				runs++
				send(map[string]any{"type": "run", "id": id, "code": code})
				if o, err := a.Run(ctx, code); err != nil {
					out.Error = err.Error()
				} else {
					out = o
				}
			}
			send(map[string]any{"type": "result", "id": id, "result": out.Result, "logs": out.Logs, "error": out.Error})
			b, _ := json.Marshal(out)
			res.Content = append(res.Content, chatBlock{"type": "tool_result", "tool_use_id": id, "content": string(b), "is_error": out.Error != ""})
		}
		msgs = append(msgs, res)
		if err := a.chatAppend(ctx, c.ID, res); err != nil {
			fail(err)
			return
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func chatCode(in any) (string, bool) {
	m, ok := in.(map[string]any)
	if !ok {
		return "", false
	}
	code, ok := m["code"].(string)
	return code, ok && strings.TrimSpace(code) != ""
}

func chatSystem(ac *Actor) string {
	who := ac.Email
	if ac.Name != "" {
		who = ac.Name + " <" + ac.Email + ">"
	}
	s := "Today is " + time.Now().UTC().Format("Monday 2 January 2006") + ". You are the agent in Archive, talking with " + who + "."
	return s + " The run tool is the only way to read or change their archive. Run calls and their results are folded away in the chat, so the answer itself presents what they need, as a short markdown table when that helps."
}

func chatPartialCode(s string) string {
	i := strings.Index(s, `"code"`)
	if i < 0 {
		return ""
	}
	rest := strings.TrimLeft(s[i+len(`"code"`):], " \t\r\n")
	rest, ok := strings.CutPrefix(rest, ":")
	if !ok {
		return ""
	}
	rest, ok = strings.CutPrefix(strings.TrimLeft(rest, " \t\r\n"), `"`)
	if !ok {
		return ""
	}
	end := len(rest)
	for j := 0; j < len(rest); j++ {
		if rest[j] == '"' {
			end = j
			break
		}
		if rest[j] == '\\' {
			n := 2
			if j+1 < len(rest) && rest[j+1] == 'u' {
				n = 6
			}
			if j+n > len(rest) {
				end = j
				break
			}
			j += n - 1
		}
	}
	var out string
	json.Unmarshal([]byte(`"`+rest[:end]+`"`), &out)
	return out
}

func chatPost(ctx context.Context, key string, body []byte) (*http.Response, error) {
	for try := 0; ; try++ {
		req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(cmp.Or(os.Getenv("ANTHROPIC_BASE_URL"), "https://api.anthropic.com"), "/")+"/v1/messages", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", chatAPIVersion)
		req.Header.Set("content-type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !retry || try == 2 {
			var e struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			json.Unmarshal(raw, &e)
			return nil, fmt.Errorf("model: %s", cmp.Or(e.Error.Message, resp.Status))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(try+1) * time.Second):
		}
	}
}

func chatTurn(ctx context.Context, key string, tools []any, system string, msgs []chatMessage, send func(map[string]any)) (*chatMessage, string, error) {
	b, err := json.Marshal(map[string]any{
		"model":         chatModel,
		"max_tokens":    chatMaxTokens,
		"stream":        true,
		"system":        []any{map[string]string{"type": "text", "text": system}},
		"tools":         tools,
		"messages":      msgs,
		"cache_control": map[string]string{"type": "ephemeral"},
	})
	if err != nil {
		return nil, "", err
	}
	resp, err := chatPost(ctx, key, b)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	var blocks []chatBlock
	var partial []string
	var stop string
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok {
			continue
		}
		var ev struct {
			Type         string         `json:"type"`
			Index        int            `json:"index"`
			ContentBlock chatBlock      `json:"content_block"`
			Delta        map[string]any `json:"delta"`
			Error        struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "content_block_start":
			for len(blocks) <= ev.Index {
				blocks, partial = append(blocks, nil), append(partial, "")
			}
			blocks[ev.Index] = ev.ContentBlock
			switch ev.ContentBlock["type"] {
			case "tool_use":
				send(map[string]any{"type": "tool", "id": ev.ContentBlock["id"]})
			case "thinking", "redacted_thinking":
				send(map[string]any{"type": "thinking"})
			}
		case "content_block_delta":
			if ev.Index >= len(blocks) || blocks[ev.Index] == nil {
				continue
			}
			blk := blocks[ev.Index]
			str := func(m map[string]any, k string) string { v, _ := m[k].(string); return v }
			switch ev.Delta["type"] {
			case "text_delta":
				blk["text"] = str(blk, "text") + str(ev.Delta, "text")
				send(map[string]any{"type": "text", "text": str(ev.Delta, "text")})
			case "thinking_delta":
				blk["thinking"] = str(blk, "thinking") + str(ev.Delta, "thinking")
			case "signature_delta":
				blk["signature"] = str(blk, "signature") + str(ev.Delta, "signature")
			case "input_json_delta":
				before := chatPartialCode(partial[ev.Index])
				partial[ev.Index] += str(ev.Delta, "partial_json")
				if code := chatPartialCode(partial[ev.Index]); code != before {
					send(map[string]any{"type": "code", "id": blk["id"], "code": code})
				}
			}
		case "content_block_stop":
			if ev.Index < len(blocks) && blocks[ev.Index] != nil && blocks[ev.Index]["type"] == "tool_use" {
				var in any
				if json.Unmarshal([]byte(partial[ev.Index]), &in) != nil {
					in = map[string]any{}
				}
				blocks[ev.Index]["input"] = in
			}
		case "message_delta":
			if s, ok := ev.Delta["stop_reason"].(string); ok {
				stop = s
			}
		case "error":
			return nil, "", errors.New("model: " + ev.Error.Message)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, "", err
	}
	if stop == "" {
		return nil, "", errors.New("model: the stream ended early")
	}
	m := &chatMessage{Role: "assistant", Content: []chatBlock{}}
	for _, blk := range blocks {
		if blk != nil {
			m.Content = append(m.Content, blk)
		}
	}
	return m, stop, nil
}
