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
)

func (a *Archive) chat(w http.ResponseWriter, r *http.Request) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		httpWrite(w, http.StatusServiceUnavailable, map[string]string{"error": "the agent needs ANTHROPIC_API_KEY set on the server"}, nil)
		return
	}
	var body struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := httpDecode(r, &body); err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	if len(body.Messages) == 0 {
		httpWrite(w, 0, nil, fmt.Errorf("messages: want the conversation: %w", httpErrBadRequest))
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	send := func(v any) {
		enc.Encode(v)
		http.NewResponseController(w).Flush()
	}
	ctx := r.Context()
	tools := []any{map[string]any{
		"name":        "run",
		"description": a.RunDocs(ctx),
		"input_schema": map[string]any{
			"type":                 "object",
			"properties":           map[string]any{"code": map[string]any{"type": "string", "description": "TypeScript to run inside Archive against the archive SDK in the tool description"}},
			"required":             []string{"code"},
			"additionalProperties": false,
		},
		"cache_control": map[string]string{"type": "ephemeral"},
	}}
	msgs := body.Messages
	for {
		msg, stop, err := chatTurn(ctx, key, tools, msgs, func(s string) { send(map[string]string{"text": s}) }, func() { send(map[string]bool{"tool": true}) })
		if err != nil {
			send(map[string]string{"error": err.Error()})
			return
		}
		b, _ := json.Marshal(msg)
		msgs = append(msgs, b)
		send(map[string]any{"message": msg})
		if stop != "tool_use" {
			return
		}
		var results []any
		for _, c := range msg["content"].([]any) {
			blk, _ := c.(map[string]any)
			if blk["type"] != "tool_use" {
				continue
			}
			var in struct {
				Code string `json:"code"`
			}
			raw, _ := json.Marshal(blk["input"])
			json.Unmarshal(raw, &in)
			res := map[string]any{"type": "tool_result", "tool_use_id": blk["id"]}
			out, err := a.Run(ctx, in.Code)
			if err != nil {
				res["content"], res["is_error"] = err.Error(), true
			} else {
				ob, _ := json.Marshal(out)
				res["content"], res["is_error"] = string(ob), out.Error != ""
			}
			results = append(results, res)
		}
		user := map[string]any{"role": "user", "content": results}
		b, _ = json.Marshal(user)
		msgs = append(msgs, b)
		send(map[string]any{"message": user})
	}
}

func chatTurn(ctx context.Context, key string, tools []any, msgs []json.RawMessage, text func(string), tool func()) (map[string]any, string, error) {
	b, err := json.Marshal(map[string]any{
		"model":      cmp.Or(os.Getenv("ARCHIVE_MODEL"), "claude-haiku-5-5"),
		"max_tokens": 32000,
		"stream":     true,
		"tools":      tools,
		"messages":   msgs,
	})
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", cmp.Or(os.Getenv("ANTHROPIC_BASE_URL"), "https://api.anthropic.com")+"/v1/messages", bytes.NewReader(b))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		raw, _ := io.ReadAll(resp.Body)
		json.Unmarshal(raw, &e)
		return nil, "", fmt.Errorf("model: %s", cmp.Or(e.Error.Message, resp.Status))
	}
	var blocks []map[string]any
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
			ContentBlock map[string]any `json:"content_block"`
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
			if ev.ContentBlock["type"] == "tool_use" {
				tool()
			}
		case "content_block_delta":
			if ev.Index >= len(blocks) {
				continue
			}
			blk := blocks[ev.Index]
			s := func(k string) string { v, _ := ev.Delta[k].(string); return v }
			switch ev.Delta["type"] {
			case "text_delta":
				t, _ := blk["text"].(string)
				blk["text"] = t + s("text")
				text(s("text"))
			case "thinking_delta":
				t, _ := blk["thinking"].(string)
				blk["thinking"] = t + s("thinking")
			case "signature_delta":
				t, _ := blk["signature"].(string)
				blk["signature"] = t + s("signature")
			case "input_json_delta":
				partial[ev.Index] += s("partial_json")
			}
		case "content_block_stop":
			if ev.Index < len(blocks) && partial[ev.Index] != "" {
				var in any
				if err := json.Unmarshal([]byte(partial[ev.Index]), &in); err == nil {
					blocks[ev.Index]["input"] = in
				}
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
	content := []any{}
	for _, blk := range blocks {
		if blk != nil {
			content = append(content, blk)
		}
	}
	return map[string]any{"role": "assistant", "content": content}, stop, nil
}
