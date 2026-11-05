package main

import (
	"cmp"
	"context"
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
	lastIn    int    // the input tokens of the last call
}

func (r *runner) doSDK(o *goja.Object) {
	o.Set("do", func(call goja.FunctionCall) goja.Value { return r.promise(r.do(call)) })
}

// do is an agent's loop with no conversation or memory, with every run tool call joining this run's transaction.
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
	}
	if err := r.arg(call, 1, "do", "opts", &opts); err != nil {
		return nil, err
	}
	key, err := modelKey("ANTHROPIC_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("archive.do: %w", err)
	}
	maxRuns := min(doMaxRuns, cmp.Or(opts.MaxRuns, agentMaxRuns))
	parts, err := r.modelFiles(opts.Files)
	if err != nil {
		return nil, fmt.Errorf("archive.do: %w", err)
	}
	first := agentMessage{Role: "user", Content: []agentBlock{{"type": "text", "text": goal}}}
	for _, p := range parts {
		first.Content = append(first.Content, modelBlock(p))
	}
	system := agentSystemBlocks(r.a.RunTypes(r.ctx), strings.TrimSpace(r.a.RunConnections(r.ctx)+"\n\n"+opts.Instructions+"\n\n"+
		"The run tool runs inside the transaction of the script that called you, so what it writes is saved or undone with that script, and your final text answer is returned to that script."))
	resume, err := r.clock.pause(&r.clock.do, doTimeout, "run agents")
	if err != nil {
		return nil, fmt.Errorf("archive.do: %w", err)
	}
	defer resume()
	ctx, cancel := context.WithTimeout(r.ctx, r.clock.do)
	defer cancel()
	run := func(code string) (*RunOutput, error) { return r.nested(ctx, code, nil) }
	res, err := agentLoop(ctx, key, cmp.Or(opts.Model, agentModel), system, []agentMessage{first}, maxRuns, func(map[string]any) {}, func(agentMessage) error { return nil }, run)
	if err != nil {
		return nil, fmt.Errorf("archive.do: %w", err)
	}
	return res, nil
}
