package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/chromedp/chromedp/remote"
	"github.com/dop251/goja"
)

const (
	browseTimeout = 5 * time.Minute
	browseReserve = 30 * time.Second
	browseSteps   = 40
	browseSure    = 0.7
	browseJev     = "jev-1.14.0"
)

type BrowseResult struct {
	Done   bool   `json:"done"`
	Answer string `json:"answer"`
	URL    string `json:"url"`
}

type browseEl struct {
	Tag   string  `json:"tag"`
	Text  string  `json:"text"`
	Href  string  `json:"href"`
	Value string  `json:"value"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
}

func (e browseEl) String() string {
	s := e.Tag + " " + strconv.Quote(e.Text)
	if e.Value != "" {
		s += " value " + strconv.Quote(e.Value)
	}
	if e.Href != "" {
		s += " to " + e.Href
	}
	return s
}

type browseView struct {
	URL   string     `json:"url"`
	Title string     `json:"title"`
	Text  string     `json:"text"`
	Els   []browseEl `json:"els"`
}

// list numbers the elements as Jev and Claude see them.
func (v *browseView) list() []string {
	out := make([]string, len(v.Els))
	for i, e := range v.Els {
		out[i] = strconv.Itoa(i+1) + ". " + e.String()
	}
	return out
}

type browseAct struct {
	Action   string `json:"action"`
	Mark     int    `json:"mark"`
	Text     string `json:"text"`
	Enter    bool   `json:"enter"`
	URL      string `json:"url"`
	Answer   string `json:"answer"`
	Achieved bool   `json:"achieved"`
}

type browser struct {
	ctx      context.Context
	goal     string
	jev, ant string
	history  []string
}

// runClock is a run's time budget: browse stops the run's clock and spends its own.
type runClock struct {
	t      *time.Timer
	end    time.Time
	browse time.Duration
}

func (r *runner) browseSDK(o *goja.Object) {
	o.Set("browse", func(call goja.FunctionCall) goja.Value { return r.promise(r.browse(call)) })
}

func (r *runner) browse(call goja.FunctionCall) (any, error) {
	goal, err := r.str(call, 0, "browse", "goal")
	if err != nil {
		return nil, err
	}
	var opts struct {
		URL string `json:"url"`
	}
	if err := r.arg(call, 1, "browse", "opts", &opts); err != nil {
		return nil, err
	}
	addr, jev, ant := os.Getenv("ARCHIVE_BROWSER"), strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")), strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if addr == "" || jev == "" || ant == "" {
		return nil, errors.New("archive.browse: browse is not configured on this server; it needs ARCHIVE_BROWSER, TYPESAFE_API_KEY and ANTHROPIC_API_KEY")
	}
	tx, err := r.a.st.db(r.ctx).TxWrote(r.ctx)
	if err != nil {
		return nil, err
	}
	if tx.Wrote {
		return nil, errors.New("archive.browse: this run already wrote or migrated; browse before you write: call archive.browse first, then save what it found")
	}
	c := r.clock
	if c.browse <= 0 {
		return nil, fmt.Errorf("archive.browse: this run already browsed for %s, its limit", browseTimeout)
	}
	if !c.t.Stop() {
		return nil, errors.New("archive.browse: the run timed out")
	}
	left, start := time.Until(c.end), time.Now()
	defer func() {
		c.browse -= time.Since(start)
		c.end = time.Now().Add(left)
		c.t.Reset(left)
	}()
	ctx, cancel := context.WithTimeout(r.ctx, c.browse)
	defer cancel()
	res, err := browseRun(ctx, addr, jev, ant, goal, opts.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: browse failed after %s: %v\n", time.Since(start).Round(time.Millisecond), err)
		return nil, fmt.Errorf("archive.browse: %w", err)
	}
	fmt.Fprintf(os.Stderr, "archive: browse took %s, done %v\n", time.Since(start).Round(time.Millisecond), res.Done)
	return res, nil
}

func browseRun(ctx context.Context, addr, jev, ant, goal, start string) (*BrowseResult, error) {
	ws, h, err := browseDial(ctx, addr)
	if err != nil {
		return nil, err
	}
	actx, cancel := remote.NewAllocator(ctx, ws, remote.NoModifyURL, remote.WithDialHTTPHeader(h))
	defer cancel()
	bctx, cancel := chromedp.NewContext(actx)
	defer cancel()
	if err := chromedp.Do(bctx, chromedp.EmulateViewport(1280, 800)); err != nil {
		return nil, fmt.Errorf("could not start the browser: %w", err)
	}
	b := &browser{ctx: bctx, goal: goal, jev: jev, ant: ant}
	if start != "" {
		if err := b.do(chromedp.Navigate(start)); err != nil {
			return nil, fmt.Errorf("could not open %s: %w", start, err)
		}
	}
	deadline, _ := ctx.Deadline()
	lctx, lcancel := context.WithDeadline(bctx, deadline.Add(-browseReserve))
	defer lcancel()
	for step := 0; step < browseSteps && lctx.Err() == nil; step++ {
		v, err := b.look(lctx)
		if err != nil {
			if lctx.Err() != nil {
				break
			}
			return nil, err
		}
		act, err := b.next(lctx, v)
		if err != nil {
			if lctx.Err() != nil {
				break
			}
			return nil, err
		}
		if act.Action == "done" {
			return &BrowseResult{Done: act.Achieved, Answer: act.Answer, URL: v.URL}, nil
		}
		b.act(lctx, v, act)
	}
	v, err := b.look(bctx)
	if err != nil {
		return nil, err
	}
	act, err := b.plan(bctx, v, "You are out of time or steps: answer now with done, from what you found so far.")
	if err != nil {
		return nil, err
	}
	return &BrowseResult{Done: act.Action == "done" && act.Achieved, Answer: act.Answer, URL: v.URL}, nil
}

// browseID caches the ID token for the browser service; Google's last an hour.
var browseID struct {
	sync.Mutex
	tok string
	exp time.Time
}

func browseDial(ctx context.Context, addr string) (string, http.Header, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", nil, fmt.Errorf("ARCHIVE_BROWSER: %w", err)
	}
	h := http.Header{}
	if host := u.Hostname(); u.Scheme == "https" && host != "localhost" && host != "127.0.0.1" {
		browseID.Lock()
		defer browseID.Unlock()
		if browseID.tok == "" || time.Now().After(browseID.exp) {
			tok, err := gcpMetadata().get(ctx, "instance/service-accounts/default/identity?audience="+url.QueryEscape(u.Scheme+"://"+u.Host))
			if err != nil {
				return "", nil, fmt.Errorf("browser identity token: %w", err)
			}
			browseID.tok, browseID.exp = tok, time.Now().Add(45*time.Minute)
		}
		h.Set("Authorization", "Bearer "+browseID.tok)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	return u.String(), h, nil
}

func (b *browser) do(a chromedp.Action[chromedp.Void]) error {
	ctx, cancel := context.WithTimeout(b.ctx, 20*time.Second)
	defer cancel()
	_, err := chromedp.Run(ctx, a)
	return err
}

const browseJS = `(() => {
  for (const a of document.querySelectorAll('a[target]')) a.removeAttribute('target');
  const sel = 'a[href],button,input:not([type=hidden]),select,textarea,summary,[role=button],[role=link],[role=tab],[role=menuitem],[role=checkbox],[role=radio],[role=option],[role=switch],[role=searchbox],[role=textbox],[role=combobox],[onclick],[contenteditable=""],[contenteditable=true]';
  const els = [];
  for (const el of document.querySelectorAll(sel)) {
    const r = el.getBoundingClientRect();
    const x = Math.max(r.left, 0), y = Math.max(r.top, 0), w = Math.min(r.right, innerWidth) - x, h = Math.min(r.bottom, innerHeight) - y;
    if (w < 4 || h < 4) continue;
    const s = getComputedStyle(el);
    if (s.visibility === 'hidden' || +s.opacity === 0) continue;
    const hit = document.elementFromPoint(x + w / 2, y + h / 2);
    if (!hit || !(el.contains(hit) || hit.contains(el) || (el.labels && [...el.labels].some(l => l.contains(hit))))) continue;
    const field = /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName);
    const text = (el.getAttribute('aria-label') || (field ? '' : el.innerText) || el.placeholder || el.title || el.alt || el.name || '').replace(/\s+/g, ' ').trim().slice(0, 80);
    let tag = el.getAttribute('role') || el.tagName.toLowerCase();
    if (tag === 'input') tag += ' ' + (el.type || 'text');
    else if (tag === 'a') tag = 'link';
    const href = el.href && !String(el.href).startsWith('javascript:') ? String(el.href).slice(0, 120) : '';
    const value = field && el.type !== 'password' ? String(el.value || '').slice(0, 80) : '';
    els.push({ tag, text, href, value, x, y, w, h });
    if (els.length >= 150) break;
  }
  const text = (document.body ? document.body.innerText : '').replace(/[ \t]+/g, ' ').replace(/\s*\n\s*/g, '\n').trim().slice(0, 4000);
  return { url: location.href, title: document.title, text, els };
})()`

func (b *browser) look(ctx context.Context) (*browseView, error) {
	var v browseView
	var err error
	for try := 0; try < 3; try++ {
		if v, err = chromedp.Run(ctx, chromedp.Evaluate[browseView](browseJS)); err == nil {
			break
		}
		chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
	}
	if err != nil {
		return nil, fmt.Errorf("reading the page: %w", err)
	}
	return &v, nil
}

type jevAnswer struct {
	Choice     string  `json:"choice"`
	Confidence float64 `json:"confidence"`
	Noul       float64 `json:"noul"`
}

var browseActions = map[string]string{
	"click":        "Click one numbered element: a link, button, tab, checkbox or menu item",
	"type":         "Type into one numbered input, search box or text area",
	"scroll_down":  "Scroll down to see more of the page",
	"scroll_up":    "Scroll back up",
	"back":         "Go back to the previous page",
	"key_enter":    "Press Enter to submit what was just typed",
	"done":         "`goal` is achieved, or what it asks for is on `page` now",
	"need_planner": "None of these fits: the page is blocked or confusing, or `goal` needs another website",
}

// next asks Jev for the next action and hands off to Claude when it needs text or Jev is unsure.
func (b *browser) next(ctx context.Context, v *browseView) (*browseAct, error) {
	if len(v.Els) == 0 && (v.URL == "" || v.URL == "about:blank") {
		return b.plan(ctx, v, "")
	}
	marks := map[string]any{"none": "No element fits"}
	for i := range v.Els {
		marks[strconv.Itoa(i+1)] = nil
	}
	a, err := b.ask(ctx, map[string]any{
		"model": browseJev,
		"state": map[string]any{"goal": b.goal, "history": b.history, "page": map[string]string{"title": v.Title, "url": v.URL, "text": v.Text}, "elements": v.list()},
		"questions": map[string]any{
			"next":   map[string]any{"type": "choice", "instructions": "A browser shows `page`, and `elements` are the numbered elements on it that can be used. Given `history`, what is the single best next action toward `goal`?", "criteria": browseActions},
			"target": map[string]any{"type": "choice", "instructions": "Which numbered element in `elements` is the one to click or type into next, to make progress on `goal`?", "criteria": marks},
			"done":   map[string]any{"type": "noul", "instructions": "`goal` is achieved, or the answer it asks for is in `page`."},
		},
	})
	if err != nil {
		return b.plan(ctx, v, "")
	}
	next, target := a["next"], a["target"]
	n, _ := strconv.Atoi(target.Choice)
	switch {
	case a["done"].Noul >= 0.5 || next.Choice == "done":
		return b.plan(ctx, v, "A quick check says the goal looks done: if so, use done and answer from the page.")
	case next.Choice == "type" || next.Choice == "need_planner" || next.Confidence < browseSure,
		next.Choice == "click" && (n < 1 || n > len(v.Els) || target.Confidence < browseSure):
		return b.plan(ctx, v, "")
	}
	return &browseAct{Action: next.Choice, Mark: n}, nil
}

func (b *browser) ask(ctx context.Context, req map[string]any) (map[string]jevAnswer, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	for try := 0; ; try++ {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		hr, _ := http.NewRequestWithContext(cctx, "POST", "https://api.typesafe.ai/v1/systemone", bytes.NewReader(body))
		hr.Header.Set("Authorization", "Bearer "+b.jev)
		hr.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(hr)
		var out struct {
			Answers map[string]jevAnswer `json:"answers"`
		}
		status := 0
		if err == nil {
			status = resp.StatusCode
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if status == http.StatusOK {
				err = json.Unmarshal(raw, &out)
				cancel()
				return out.Answers, err
			}
			err = fmt.Errorf("jev: %s: %.200s", resp.Status, raw)
		}
		cancel()
		if status != 0 && status != 429 && status != 503 && status != 529 || try == 3 || ctx.Err() != nil {
			return nil, err
		}
		time.Sleep(time.Duration(200<<try+rand.IntN(200)) * time.Millisecond)
	}
}

const browseSystem = `You drive a web browser for a script, to reach its goal. You get the page's text and the numbered elements on it that you can use. History lists the actions already taken, oldest first, and the page is where they led. Pick the one next action with the act tool:
- click or type use an element number; type clicks the field itself, replaces its text, and with enter: true submits it.
- goto opens an address, for a start or when this site is a dead end.
- done ends: achieved says whether the goal was reached, and answer gives what the goal asks for, taken from the page, short and exact; when it cannot be reached, say why.`

var browseTool = map[string]any{
	"name":        "act",
	"description": "The next browser action, or done with the answer.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":   map[string]any{"type": "string", "enum": []string{"click", "type", "scroll_down", "scroll_up", "back", "key_enter", "goto", "done"}},
			"mark":     map[string]any{"type": "integer", "description": "For click and type: the element number"},
			"text":     map[string]any{"type": "string", "description": "For type: the text to type"},
			"enter":    map[string]any{"type": "boolean", "description": "For type: press Enter after typing"},
			"url":      map[string]any{"type": "string", "description": "For goto: the address"},
			"answer":   map[string]any{"type": "string", "description": "For done: the answer to the goal"},
			"achieved": map[string]any{"type": "boolean", "description": "For done: whether the goal was reached"},
		},
		"required": []string{"action"},
	},
}

// plan asks Claude for the next action, for text Jev cannot write and choices it is unsure of.
func (b *browser) plan(ctx context.Context, v *browseView, hint string) (*browseAct, error) {
	var s strings.Builder
	fmt.Fprintf(&s, "Goal: %s\nNow on: %s %s\nHistory:\n", b.goal, v.Title, v.URL)
	for _, h := range b.history {
		s.WriteString("- " + h + "\n")
	}
	s.WriteString("Elements:\n" + strings.Join(v.list(), "\n") + "\nPage text:\n" + v.Text + "\n" + hint)
	body, err := json.Marshal(map[string]any{
		"model":       chatModel,
		"max_tokens":  2048,
		"system":      browseSystem,
		"tools":       []any{browseTool},
		"tool_choice": map[string]any{"type": "tool", "name": "act", "disable_parallel_tool_use": true},
		"messages":    []any{map[string]string{"role": "user", "content": s.String()}},
	})
	if err != nil {
		return nil, err
	}
	resp, err := chatPost(ctx, b.ant, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Content []struct {
			Type  string    `json:"type"`
			Input browseAct `json:"input"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	for _, c := range out.Content {
		if c.Type == "tool_use" {
			return &c.Input, nil
		}
	}
	return nil, errors.New("model: no action")
}

func (b *browser) act(ctx context.Context, v *browseView, a *browseAct) {
	desc := a.Action
	var el *browseEl
	if a.Mark >= 1 && a.Mark <= len(v.Els) {
		el = &v.Els[a.Mark-1]
		desc += " " + el.String()
	}
	var err error
	switch a.Action {
	case "click", "type":
		if el == nil {
			err = fmt.Errorf("there is no element %d", a.Mark)
			break
		}
		err = b.do(chromedp.MouseClickXY(el.X+el.W/2, el.Y+el.H/2))
		if err == nil && a.Action == "type" {
			desc += " <- " + strconv.Quote(a.Text)
			err = b.do(chromedp.Steps(
				chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
					_, err := chromedp.Evaluate[any](`(() => { const e = document.activeElement; if (e && e.select) e.select(); else if (e && e.isContentEditable) document.execCommand('selectAll'); })()`)(ctx, t)
					return err
				}),
				chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
					_, err := cdp.Call(ctx, t, input.InsertText, input.InsertTextParams{Text: a.Text})
					return err
				})))
			if err == nil && a.Enter {
				desc += " + enter"
				err = b.do(chromedp.KeyEvent(kb.Enter))
			}
		}
	case "scroll_down", "scroll_up":
		dy := 600.0
		if a.Action == "scroll_up" {
			dy = -dy
		}
		err = b.do(chromedp.MouseEvent(input.DispatchMouseEventTypeMouseWheel, 640, 400, func(p *input.DispatchMouseEventParams) { p.DeltaY = dy }))
	case "back":
		err = b.do(chromedp.NavigateBack())
	case "key_enter":
		err = b.do(chromedp.KeyEvent(kb.Enter))
	case "goto":
		desc += " " + a.URL
		err = b.do(chromedp.Navigate(a.URL))
	default:
		err = errors.New("unknown action")
	}
	chromedp.Run(ctx, chromedp.Sleep(400*time.Millisecond))
	for i := 0; i < 25 && ctx.Err() == nil; i++ {
		if s, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`document.readyState`)); s == "complete" {
			break
		}
		chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))
	}
	if err != nil {
		desc += " (failed: " + err.Error() + ")"
	} else if u, _ := chromedp.Run(ctx, chromedp.Location()); u != "" && u != v.URL {
		desc += ", which opened " + u
	}
	b.history = append(b.history, "on "+v.URL+": "+desc)
	if len(b.history) > 12 {
		b.history = b.history[len(b.history)-12:]
	}
}
