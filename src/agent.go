package main

// Agents are rows of archive.agent, which the workspace or the group in their access sees. Each person's conversation with one is
// theirs alone and never ends: a request reads the turns still in context, word for word, and one line of memory
// for everything older, and answers as that person, under their role.
//
// Memory is a binary tree over the turns (archive.sql). A turn leaves the context when a call's input passes
// memoryAt, and then gets its line; lines pair up into summaries of 2, 4, 8... turns. What the model reads is
// cover: at most memoryLines lines, the oldest turns in the largest summaries. It changes only when turns leave
// the context, so the prompt cache holds between. Nothing is lost, since the agent can read the turn and memory
// tables back to the first word. What is missing is worked out from the rows, so a failed compaction just
// finishes after the next turn.

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	agentMaxTokens = 64000
	agentMaxRuns   = 25
	agentModel     = "claude-haiku-5-5"

	memoryLines  = 96      // the lines of memory in the context
	memoryAt     = 100_000 // a call's input tokens past which turns leave the context
	memoryKeep   = 100_000 // the bytes of newest turns that stay, about a quarter of that
	memoryLine   = 280     // the bytes of one line
	memoryResult = 2000    // the bytes of a run's result a turn's line is made from
	memoryRaw    = 16      // a summary of up to this many turns is made from their lines, a larger one from its halves
	memoryCalls  = 8       // the model calls compaction makes at once
)

// Turn is a turn as the app reads it, under row-level security, so only the caller's own.
type Turn struct {
	N        int64           `json:"n"`
	Messages json.RawMessage `json:"messages"`
}

func (a *Archive) Turns(ctx context.Context, agent uuid.UUID, limit int) ([]Turn, error) {
	out := []Turn{}
	if limit <= 0 {
		limit = 20
	}
	return out, a.as(ctx, func(ctx context.Context) error {
		rows, err := a.st.db(ctx).LastTurns(ctx, agent, int32(limit))
		for _, r := range rows {
			out = append(out, Turn{N: r.N, Messages: r.Messages})
		}
		return err
	})
}

// ClearTurns forgets the caller's conversation with an agent, its memory of them included. The agent and everyone
// else's conversations with it stay.
func (a *Archive) ClearTurns(ctx context.Context, agent uuid.UUID) error {
	ac, err := signedIn(ctx)
	if err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).ClearTurns(ctx, agent, ac.member)
	})
}

// turnCreate starts the asker's next turn, named after what they said. Two at once, from two tabs, simply get the
// next numbers.
func (a *Archive) turnCreate(ctx context.Context, agent, member uuid.UUID, text string, first agentMessage) (uuid.UUID, error) {
	b, err := json.Marshal([]agentMessage{first})
	if err != nil {
		return uuid.Nil, err
	}
	name := strings.Join(strings.Fields(text), " ")
	if r := []rune(name); len(r) > 60 {
		name = string(r[:60]) + "…"
	}
	for try := 0; ; try++ {
		var id uuid.UUID
		err := a.sys(ctx, func(ctx context.Context) (err error) {
			id, err = a.st.db(ctx).CreateTurn(ctx, generated.CreateTurnParams{Agent: agent, Member: member, Name: name, Messages: b})
			return err
		})
		if !errors.Is(err, pgx.ErrNoRows) || try == 4 {
			return id, err
		}
	}
}

type agentBlock = map[string]any

type agentMessage struct {
	Role    string       `json:"role"`
	Content []agentBlock `json:"content"`
}

func agentUses(m agentMessage) []agentBlock {
	var out []agentBlock
	for _, b := range m.Content {
		if b["type"] == "tool_use" {
			out = append(out, b)
		}
	}
	return out
}

func agentToolError(uses []agentBlock, msg string) *agentMessage {
	if len(uses) == 0 {
		return nil
	}
	m := &agentMessage{Role: "user"}
	for _, u := range uses {
		m.Content = append(m.Content, agentBlock{"type": "tool_result", "tool_use_id": u["id"], "content": msg, "is_error": true})
	}
	return m
}

// turn answers one message, streaming one JSON event per line, then compacts with the stream still open: the app
// has its answer at done, and the open request keeps the instance working.
func (a *Archive) turn(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if key == "" {
		httpWrite(w, http.StatusServiceUnavailable, map[string]string{"error": "the agent needs ANTHROPIC_API_KEY set on the server"}, nil)
		return
	}
	var body struct {
		Agent uuid.UUID `json:"agent"`
		Text  string    `json:"text"`
	}
	if err := httpDecode(r, &body); err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	ctx := context.WithValue(r.Context(), schemaTurnKey{}, uuid.NewString())
	ac := ActorFrom(ctx)
	text := strings.TrimSpace(body.Text)
	if text == "" {
		httpWrite(w, 0, nil, fmt.Errorf("text: want a message: %w", httpErrBadRequest))
		return
	}
	first := agentMessage{Role: "user", Content: []agentBlock{{"type": "text", "text": text}}}
	var memory string
	var msgs []agentMessage
	var id uuid.UUID
	agent, err := a.Get(ctx, "agent", body.Agent)
	if err == nil {
		memory, msgs, err = a.memoryRead(ctx, body.Agent, ac.member)
	}
	if err == nil {
		id, err = a.turnCreate(ctx, body.Agent, ac.member, text, first)
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
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
	save := func(m agentMessage) error {
		b, err := json.Marshal([]agentMessage{m})
		if err != nil {
			return err
		}
		return a.sys(context.WithoutCancel(ctx), func(ctx context.Context) error {
			return a.st.db(ctx).AppendTurn(ctx, b, id)
		})
	}
	run := func(code string) (*RunOutput, error) {
		out, err := a.Run(ctx, code)
		if err != nil {
			out = &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: err.Error()}
		}
		return out, nil
	}
	role, _, _ := a.level(ctx)
	system := agentSystemBlocks(a.RunTypes(ctx), strings.TrimSpace(a.RunConnections(ctx))+"\n\n"+agentSystem(ac, role, agent, memory))
	res, err := agentLoop(ctx, key, agentModel, system, append(msgs, first), agentMaxRuns, send, save, run)
	if err != nil {
		send(map[string]any{"type": "error", "error": err.Error()})
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()
	if err := a.memoryCompact(ctx, body.Agent, ac.member, res.lastIn); err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s memory of agent %s: %v\n", a.ws.ID, body.Agent, err)
	}
}

// agentLoop runs the model and its run calls until it answers, for a turn and for archive.do: send hears the events,
// save keeps each message as it comes, and an error from run ends it. What it has so far comes back with any error.
func agentLoop(ctx context.Context, key, model string, system []any, msgs []agentMessage, maxRuns int, send func(map[string]any), save func(agentMessage) error, run func(string) (*RunOutput, error)) (*DoResult, error) {
	res := &DoResult{}
	for {
		msg, stop, usage, err := agentCall(ctx, key, model, system, msgs, send)
		if err != nil {
			return res, err
		}
		res.TokensIn, res.TokensOut, res.lastIn = res.TokensIn+usage.In, res.TokensOut+usage.Out, usage.In
		msgs = append(msgs, msg)
		if err := save(msg); err != nil {
			return res, err
		}
		uses := agentUses(msg)
		if stop != "tool_use" || len(uses) == 0 {
			if m := agentToolError(uses, "the answer was cut off before this call was complete"); m != nil {
				save(*m)
			}
			switch stop {
			case "refusal":
				return res, errors.New("the model declined to answer this")
			case "max_tokens":
				return res, errors.New("the answer hit the length limit")
			}
			send(map[string]any{"type": "done", "stop": stop})
			for _, b := range msg.Content {
				if t, ok := b["text"].(string); ok && b["type"] == "text" {
					res.Answer += t
				}
			}
			return res, nil
		}
		reply := agentMessage{Role: "user"}
		for _, u := range uses {
			in, _ := u["input"].(map[string]any)
			code, ok := in["code"].(string)
			out := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}
			switch {
			case !ok || strings.TrimSpace(code) == "":
				out.Error = "the input was not an object with a code string; call run again"
			case res.Runs >= maxRuns:
				out.Error = fmt.Sprintf("this answer already ran code %d times, the limit; answer with what you have", maxRuns)
			default:
				res.Runs++
				send(map[string]any{"type": "run", "id": u["id"], "description": agentDescription(u["input"]), "code": code})
				if out, err = run(code); err != nil {
					return res, err
				}
			}
			send(map[string]any{"type": "result", "id": u["id"], "result": out.Result, "logs": out.Logs, "error": out.Error, "files": out.Files})
			b, _ := json.Marshal(out)
			reply.Content = append(reply.Content, agentBlock{"type": "tool_result", "tool_use_id": u["id"], "content": string(b), "is_error": out.Error != ""})
		}
		msgs = append(msgs, reply)
		if err := save(reply); err != nil {
			return res, err
		}
	}
}

// cover tiles turns [0,t) with aligned power-of-two blocks, at most budget of them, finest near t: a block stays
// whole while its size is at most alpha times its age, with alpha as small as the budget allows, and what
// budget remains splits the newest blocks.
func cover(t, budget int64) [][2]int64 {
	tile := func(alpha float64) [][2]int64 {
		root := int64(1)
		for root < t {
			root *= 2
		}
		var out [][2]int64
		var walk func(lo, hi int64)
		walk = func(lo, hi int64) {
			if lo >= t {
				return
			}
			if size := hi - lo; size > 1 && (hi > t || float64(size) > alpha*float64(t-lo)) {
				walk(lo, (lo+hi)/2)
				walk((lo+hi)/2, hi)
				return
			}
			out = append(out, [2]int64{lo, hi})
		}
		walk(0, root)
		return out
	}
	if t <= 0 {
		return nil
	}
	lo, hi := 0.0, 1.0
	for range 60 {
		if mid := (lo + hi) / 2; int64(len(tile(mid))) > budget {
			lo = mid
		} else {
			hi = mid
		}
	}
	out := tile(hi)
	for int64(len(out)) < budget {
		i := len(out) - 1
		for i >= 0 && out[i][1]-out[i][0] == 1 {
			i--
		}
		if i < 0 {
			break
		}
		b, mid := out[i], (out[i][0]+out[i][1])/2
		out = append(out[:i], append([][2]int64{{b[0], mid}, {mid, b[1]}}, out[i+1:]...)...)
	}
	return out
}

func memoryLevels(ctx context.Context, q *generated.Queries, agent, member uuid.UUID) (map[int64]int64, error) {
	rows, err := q.MemoryLevels(ctx, agent, member)
	out := map[int64]int64{}
	for _, r := range rows {
		out[r.Size] = r.Lines
	}
	return out, err
}

// memoryRead is what the model knows of this conversation: the memory block for the turns that left the context,
// and those still in it, a call that was stopped before it ran answered as such.
func (a *Archive) memoryRead(ctx context.Context, agent, member uuid.UUID) (string, []agentMessage, error) {
	lines := map[int64]string{}
	var msgs []agentMessage
	err := a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		levels, err := memoryLevels(ctx, q, agent, member)
		if err != nil {
			return err
		}
		base := levels[1]
		// A summary still missing shows as its halves, so no turn goes unmentioned, up to twice the lines.
		for blocks := cover(base, memoryLines); len(blocks) > 0; {
			var los, sizes []int64
			for _, b := range blocks {
				los, sizes = append(los, b[0]), append(sizes, b[1]-b[0])
			}
			rows, err := q.MemoryBlocks(ctx, generated.MemoryBlocksParams{Agent: agent, Member: member, Los: los, Sizes: sizes})
			if err != nil {
				return err
			}
			have := map[[2]int64]string{}
			for _, r := range rows {
				have[[2]int64{r.Lo, r.Size}] = r.Name
			}
			var next [][2]int64
			for i, b := range blocks {
				line, ok := have[[2]int64{b[0], b[1] - b[0]}]
				label := fmt.Sprintf("#%d", b[0])
				if b[1]-b[0] > 1 {
					label = fmt.Sprintf("#%d-%d", b[0], b[1]-1)
				}
				switch {
				case ok:
					lines[b[0]] = label + " " + line
				case b[1]-b[0] > 1 && len(lines)+len(next)+len(blocks)-i < 2*memoryLines:
					mid := (b[0] + b[1]) / 2
					next = append(next, [2]int64{b[0], mid}, [2]int64{mid, b[1]})
				default:
					lines[b[0]] = label + " not summed up yet; read these turns"
				}
			}
			blocks = next
		}
		turns, err := q.TurnsBetween(ctx, generated.TurnsBetweenParams{Agent: agent, Member: member, Lo: base, Hi: 1 << 62})
		if err != nil {
			return err
		}
		for _, t := range turns {
			var ms []agentMessage
			if err := json.Unmarshal(t.Messages, &ms); err != nil {
				return err
			}
			msgs = append(msgs, ms...)
			if last := ms[len(ms)-1]; last.Role == "assistant" {
				if m := agentToolError(agentUses(last), "stopped before this ran"); m != nil {
					msgs = append(msgs, *m)
				}
			}
		}
		return nil
	})
	var memory []string
	for _, lo := range slices.Sorted(maps.Keys(lines)) {
		memory = append(memory, lines[lo])
	}
	return strings.Join(memory, "\n"), msgs, err
}

// memoryCompact moves turns out of the context when the last call took more than memoryAt tokens, keeping the
// newest memoryKeep bytes and always the last turn, then makes every line and summary still missing, in order.
func (a *Archive) memoryCompact(ctx context.Context, agent, member uuid.UUID, tokens int) error {
	var levels map[int64]int64
	var turns []generated.TurnsBetweenRow
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		q := a.st.db(ctx)
		if levels, err = memoryLevels(ctx, q, agent, member); err != nil || tokens <= memoryAt {
			return err
		}
		sizes, err := q.TurnSizes(ctx, agent, member, levels[1])
		if err != nil || len(sizes) < 2 {
			return err
		}
		keep, kept := sizes[0].N, sizes[0].Bytes
		for _, s := range sizes[1:] {
			if kept += s.Bytes; kept > memoryKeep {
				break
			}
			keep = s.N
		}
		turns, err = q.TurnsBetween(ctx, generated.TurnsBetweenParams{Agent: agent, Member: member, Lo: levels[1], Hi: keep})
		return err
	})
	if err != nil {
		return err
	}
	base := levels[1]
	n, err := a.memoryWrite(ctx, agent, member, 1, base, len(turns), func(i int) (string, error) {
		line, err := memoryAsk(ctx, "one exchange with a person, with what its run calls found", turnText(turns[i].Messages))
		return turns[i].CreatedAt.Format("2006-01-02") + " " + line, err
	})
	base += int64(n)
	if err != nil {
		return err
	}
	for size := int64(2); size <= base; size *= 2 {
		want := int(base/size - levels[size])
		from := levels[size] * size
		n, err := a.memoryWrite(ctx, agent, member, size, from, want, func(i int) (string, error) {
			lo := from + int64(i)*size
			of := size / 2
			if size <= memoryRaw {
				of = 1
			}
			var rows []generated.MemoryLevelRow
			err := a.sys(ctx, func(ctx context.Context) (err error) {
				rows, err = a.st.db(ctx).MemoryLevel(ctx, generated.MemoryLevelParams{Agent: agent, Member: member, Size: of, Lo: lo, Hi: lo + size})
				return err
			})
			if err != nil {
				return "", err
			}
			var text strings.Builder
			for _, r := range rows {
				fmt.Fprintf(&text, "#%d %s\n", r.Lo, r.Name)
			}
			return memoryAsk(ctx, "these memories", text.String())
		})
		if err != nil || n < want {
			return err
		}
	}
	return nil
}

// memoryWrite makes n lines of size from lo on, a few at a time, and saves them in order up to the first that failed,
// so what is saved of a level is always its start. It says how many it saved.
func (a *Archive) memoryWrite(ctx context.Context, agent, member uuid.UUID, size, lo int64, n int, line func(int) (string, error)) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	lines, errs := make([]string, n), make([]error, n)
	var wg sync.WaitGroup
	slots := make(chan struct{}, memoryCalls)
	for i := range n {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			lines[i], errs[i] = line(i)
		})
	}
	wg.Wait()
	for i := range n {
		err := errs[i]
		if err == nil {
			err = a.sys(ctx, func(ctx context.Context) error {
				return a.st.db(ctx).SaveMemory(ctx, generated.SaveMemoryParams{Agent: agent, Member: member, Lo: lo + int64(i)*size, Size: size, Name: lines[i]})
			})
		}
		if err != nil {
			return i, err
		}
	}
	return n, ctx.Err()
}

// memoryAsk has the model write one line. A line too long gets one more try, with the model told so, since cutting
// it would lose whatever it said last; only if that is too long as well is it cut.
func memoryAsk(ctx context.Context, what, text string) (string, error) {
	limit := memoryLine - len("2006-01-02 ")
	ask := func(prompt string) (string, error) {
		res, err := modelClaude(ctx, &modelReq{Model: agentModel, Prompt: prompt}, nil)
		if err != nil {
			return "", err
		}
		line := strings.Join(strings.Fields(res.Text), " ")
		if line == "" {
			return "", errors.New("memory: the model wrote nothing")
		}
		return line, nil
	}
	prompt := fmt.Sprintf("Compress %s into one line of at most %d bytes, in the third person, with no preamble.\nKeep what has lasting effect, drop what does not. Invent nothing.\n\n%s", what, limit, text)
	line, err := ask(prompt)
	if err != nil {
		return "", err
	}
	if len(line) > limit {
		if again, err := ask(fmt.Sprintf("%s\n\nYour last try was %d bytes, too long:\n%s\nWrite it again in at most %d bytes, keeping the facts that last.", prompt, len(line), line, limit)); err == nil {
			line = again
		}
	}
	return memoryCut(line, limit), nil
}

func memoryCut(s string, limit int) string {
	if len(s) > limit {
		return strings.ToValidUTF8(s[:limit], "")
	}
	return s
}

// turnText is a turn as its line is made from: what was said, what each run call was for and what it found.
func turnText(raw json.RawMessage) string {
	var ms []agentMessage
	json.Unmarshal(raw, &ms)
	var b strings.Builder
	for _, m := range ms {
		for _, c := range m.Content {
			switch c["type"] {
			case "text":
				fmt.Fprintf(&b, "%s: %s\n", m.Role, c["text"])
			case "tool_use":
				fmt.Fprintf(&b, "ran: %s\n", agentDescription(c["input"]))
			case "tool_result":
				out, _ := c["content"].(string)
				fmt.Fprintf(&b, "result: %s\n", memoryCut(out, memoryResult))
			}
		}
	}
	return b.String()
}

// agentTools is the agent's one tool, run, whose description goes first so a step's label streams before its code.
var agentTools = []any{map[string]any{
	"name":        "run",
	"description": runDocs,
	"input_schema": map[string]any{
		"type": "object",
		"properties": json.RawMessage(`{"description":{"type":"string","description":"What this step does, in a few words, for the person watching: 'Finding unpaid invoices', 'Adding 3 rows to contacts'"},` +
			`"code":{"type":"string","description":"TypeScript to run inside Archive against the archive SDK in this tool's description"}}`),
		"required":             []string{"description", "code"},
		"additionalProperties": false,
	},
	"eager_input_streaming": true,
	"cache_control":         map[string]string{"type": "ephemeral"},
}}

func agentDescription(in any) string {
	m, _ := in.(map[string]any)
	d, _ := m["description"].(string)
	return strings.TrimSpace(d)
}

// agentSystemBlocks is the system prompt: first the workspace's tables, as their own cache entry shared by everyone
// in the workspace; then the caller's own (text), which the conversation's breakpoint caches. Ahead of both, the
// run tool's description is the same in every workspace (runDocs), so a turn has three breakpoints, all with the
// default lifetime.
func agentSystemBlocks(types, text string) []any {
	return []any{
		map[string]any{"type": "text", "text": "The tables of this archive, as TypeScript, which the SDK in the run tool's description reads and writes:\n\n" + types,
			"cache_control": map[string]string{"type": "ephemeral"}},
		map[string]any{"type": "text", "text": text},
	}
}

// agentSystem says who the agent is, who it is talking with and their role in the workspace, which is what they
// may do, what it is for, and what it remembers of them.
func agentSystem(ac *Actor, role string, agent map[string]any, memory string) string {
	who := ac.Email
	if ac.Name != "" {
		who = ac.Name + " <" + ac.Email + ">"
	}
	s := "Today is " + time.Now().UTC().Format("Monday 2 January 2006") + ". You are " + fmt.Sprint(agent["name"]) + ", an agent in Archive, talking with " + who + "."
	switch role {
	case "owner", "editor":
		s += " They are an " + role + " of this workspace."
	case "viewer":
		s += " They are a viewer of this workspace."
	}
	s += " Your id in the agent table is " + fmt.Sprint(agent["id"]) + ". The run tool is the only way to read or change their archive. Run calls and their results are folded away in the conversation, so the answer itself presents what they need, as a short markdown table when that helps."
	if in := strings.TrimSpace(fmt.Sprint(agent["instructions"])); in != "" {
		s += "\n\nYour instructions, which the workspace set:\n" + in
	}
	s += "\n\nThis conversation has no end, and no new session starts. Older turns leave it and stay as memory, below if there is any: one line per #turn or #first-last range, oldest first, coarser with age."
	if memory != "" {
		s += "\n\nYour memory of them:\n" + memory
	}
	return s
}

// agentPartialRe finds the strings in a tool input still being written, up to what has streamed whole.
var agentPartialRe = regexp.MustCompile(`"(description|code)"\s*:\s*"((?:[^"\\]|\\[^u]|\\u[0-9a-fA-F]{4})*)`)

func agentPartial(s string) (desc, code string) {
	for _, m := range agentPartialRe.FindAllStringSubmatch(s, -1) {
		var v string
		json.Unmarshal([]byte(`"`+m[2]+`"`), &v)
		if m[1] == "code" {
			code = v
		} else {
			desc = v
		}
	}
	return desc, code
}

func agentPost(ctx context.Context, key string, body []byte) (*http.Response, error) {
	url := strings.TrimRight(cmp.Or(os.Getenv("ANTHROPIC_BASE_URL"), "https://api.anthropic.com"), "/") + "/v1/messages"
	return modelPost(ctx, url, http.Header{"x-api-key": {key}, "anthropic-version": {"2026-10-21"}}, body)
}

// modelPost posts JSON to a model API, trying three times on 429 and 5xx; Anthropic and Gemini both answer an
// error as {"error": {"message"}}.
func modelPost(ctx context.Context, url string, header http.Header, body []byte) (*http.Response, error) {
	for try := 0; ; try++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header = header.Clone()
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

// agentUsage is what one call cost: message_start carries the input tokens, split by the prompt cache, and
// message_delta the output tokens.
type agentUsage struct {
	In      int `json:"input_tokens"`
	Out     int `json:"output_tokens"`
	Cached  int `json:"cache_read_input_tokens"`
	Written int `json:"cache_creation_input_tokens"`
}

func agentCall(ctx context.Context, key, model string, system []any, msgs []agentMessage, send func(map[string]any)) (agentMessage, string, agentUsage, error) {
	var usage agentUsage
	m := agentMessage{Role: "assistant", Content: []agentBlock{}}
	b, err := json.Marshal(map[string]any{
		"model":         model,
		"max_tokens":    agentMaxTokens,
		"stream":        true,
		"system":        system,
		"tools":         agentTools,
		"messages":      msgs,
		"cache_control": map[string]string{"type": "ephemeral"},
	})
	if err != nil {
		return m, "", usage, err
	}
	resp, err := agentPost(ctx, key, b)
	if err != nil {
		return m, "", usage, err
	}
	defer resp.Body.Close()
	var stop, input, desc, code string
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok {
			continue
		}
		var ev struct {
			Type         string         `json:"type"`
			ContentBlock agentBlock     `json:"content_block"`
			Delta        map[string]any `json:"delta"`
			Message      struct {
				Usage agentUsage `json:"usage"`
			} `json:"message"`
			Usage agentUsage `json:"usage"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "message_start":
			u := ev.Message.Usage
			usage.In = u.In + u.Cached + u.Written
		case "content_block_start":
			m.Content = append(m.Content, ev.ContentBlock)
			input, desc, code = "", "", ""
			switch ev.ContentBlock["type"] {
			case "tool_use":
				send(map[string]any{"type": "tool", "id": ev.ContentBlock["id"]})
			case "thinking", "redacted_thinking":
				send(map[string]any{"type": "thinking"})
			}
		case "content_block_delta":
			blk := m.Content[len(m.Content)-1]
			for _, k := range []string{"text", "thinking", "signature"} {
				if d, ok := ev.Delta[k].(string); ok {
					s, _ := blk[k].(string)
					blk[k] = s + d
				}
			}
			if t, ok := ev.Delta["text"].(string); ok {
				send(map[string]any{"type": "text", "text": t})
			}
			if j, ok := ev.Delta["partial_json"].(string); ok {
				input += j
				if d, c := agentPartial(input); d != desc || c != code {
					desc, code = d, c
					send(map[string]any{"type": "code", "id": blk["id"], "description": strings.TrimSpace(d), "code": c})
				}
			}
		case "content_block_stop":
			if blk := m.Content[len(m.Content)-1]; blk["type"] == "tool_use" {
				var in any
				if json.Unmarshal([]byte(input), &in) != nil {
					in = map[string]any{}
				}
				blk["input"] = in
			}
		case "message_delta":
			if s, ok := ev.Delta["stop_reason"].(string); ok {
				stop = s
			}
			usage.Out = ev.Usage.Out
		case "error":
			return m, "", usage, errors.New("model: " + ev.Error.Message)
		}
	}
	if err := sc.Err(); err != nil {
		return m, "", usage, err
	}
	if stop == "" {
		return m, "", usage, errors.New("model: the stream ended early")
	}
	return m, stop, usage, nil
}

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
