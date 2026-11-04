package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

const (
	doTimeout = 10 * time.Minute
	doMaxRuns = 50
)

type DoResult struct {
	Answer    string `json:"answer"`
	Runs      int    `json:"runs"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
}

func (r *runner) doSDK(o *goja.Object) {
	o.Set("do", func(call goja.FunctionCall) goja.Value { return r.promise(r.do(call)) })
}

// do is the chat's agent loop, headless, with every run tool call joining this run's transaction.
func (r *runner) do(call goja.FunctionCall) (any, error) {
	goal, err := r.str(call, 0, "do", "goal")
	if err != nil {
		return nil, err
	}
	var opts struct {
		Model        string      `json:"model"`
		Instructions string      `json:"instructions"`
		Files        []modelFile `json:"files"`
		MaxRuns      int         `json:"maxRuns"`
		Tables       []string    `json:"tables"`
	}
	if err := r.arg(call, 1, "do", "opts", &opts); err != nil {
		return nil, err
	}
	key, err := modelKey("ANTHROPIC_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("archive.do: %w", err)
	}
	maxRuns := min(doMaxRuns, cmp.Or(opts.MaxRuns, chatMaxRuns))
	parts, err := r.modelFiles(opts.Files)
	if err != nil {
		return nil, fmt.Errorf("archive.do: %w", err)
	}
	first := chatMessage{Role: "user", Content: []chatBlock{{"type": "text", "text": goal}}}
	for _, p := range parts {
		first.Content = append(first.Content, modelBlock(p))
	}
	types, err := r.a.RunTypesFor(r.ctx, opts.Tables)
	if err != nil {
		return nil, fmt.Errorf("archive.do: %w", err)
	}
	system := chatSystemBlocks(types, strings.TrimSpace(opts.Instructions+"\n\n"+
		"The run tool runs inside the transaction of the script that called you, so what it writes is saved or undone with that script, and your final text answer is returned to that script."))
	resume, err := r.clock.pause(&r.clock.do, doTimeout, "run agents")
	if err != nil {
		return nil, fmt.Errorf("archive.do: %w", err)
	}
	defer resume()
	ctx, cancel := context.WithTimeout(r.ctx, r.clock.do)
	defer cancel()
	msgs, res := []chatMessage{first}, &DoResult{}
	for {
		msg, stop, usage, err := chatTurn(ctx, key, cmp.Or(opts.Model, chatModel), chatTools, system, msgs, func(map[string]any) {})
		if err != nil {
			return nil, fmt.Errorf("archive.do: %w", err)
		}
		res.TokensIn, res.TokensOut = res.TokensIn+usage.In, res.TokensOut+usage.Out
		msgs = append(msgs, *msg)
		uses := chatUses(*msg)
		if stop != "tool_use" || len(uses) == 0 {
			switch stop {
			case "refusal":
				return nil, errors.New("archive.do: the model declined to do this")
			case "max_tokens":
				return nil, errors.New("archive.do: the answer hit the length limit")
			}
			var text []string
			for _, b := range msg.Content {
				if t, ok := b["text"].(string); ok && b["type"] == "text" {
					text = append(text, t)
				}
			}
			res.Answer = strings.Join(text, "")
			return res, nil
		}
		reply := chatMessage{Role: "user"}
		for _, u := range uses {
			code, ok := chatCode(u["input"])
			out := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}
			switch {
			case !ok:
				out.Error = "the input was not an object with a code string; call run again"
			case res.Runs >= maxRuns:
				out.Error = fmt.Sprintf("this task already ran code %d times, the limit; answer with what you have", maxRuns)
			default:
				res.Runs++
				if out, err = r.nested(ctx, code, nil); err != nil {
					return nil, fmt.Errorf("archive.do: %w", err)
				}
			}
			b, _ := json.Marshal(out)
			reply.Content = append(reply.Content, chatBlock{"type": "tool_result", "tool_use_id": u["id"], "content": string(b), "is_error": out.Error != ""})
		}
		msgs = append(msgs, reply)
	}
}
