package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/chromedp/chromedp/remote"
	"github.com/dop251/goja"
	"github.com/google/uuid"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	browseTimeout = 5 * time.Minute
	browseReserve = 30 * time.Second
	browseSteps   = 40
	browseSure    = 0.7
	browseJev     = "jev-1.14.0"
)

type browseFrameKey struct{}

// browseFrames makes archive.browse in runs under ctx call fn with each screencast frame, a base64 JPEG.
func browseFrames(ctx context.Context, fn func(frame, url string)) context.Context {
	return context.WithValue(ctx, browseFrameKey{}, fn)
}

type BrowseResult struct {
	Done   bool   `json:"done"`
	Answer string `json:"answer"`
	URL    string `json:"url"`
}

type browseEl struct {
	Tag  string  `json:"tag"`
	Text string  `json:"text"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
}

func (e browseEl) String() string { return e.Tag + " " + strconv.Quote(e.Text) }

type browseView struct {
	URL   string     `json:"url"`
	Title string     `json:"title"`
	Els   []browseEl `json:"els"`
	shot  []byte
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
	url      atomic.Value
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
	addr, jev, ant := os.Getenv("ARCHIVE_BROWSER"), os.Getenv("TYPESAFE_API_KEY"), os.Getenv("ANTHROPIC_API_KEY")
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
	frames, _ := r.ctx.Value(browseFrameKey{}).(func(frame, url string))
	res, err := browseRun(ctx, addr, jev, ant, goal, opts.URL, frames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: browse failed after %s: %v\n", time.Since(start).Round(time.Millisecond), err)
		return nil, fmt.Errorf("archive.browse: %w", err)
	}
	fmt.Fprintf(os.Stderr, "archive: browse took %s, done %v\n", time.Since(start).Round(time.Millisecond), res.Done)
	return res, nil
}

func browseRun(ctx context.Context, addr, jev, ant, goal, start string, frames func(frame, url string)) (*BrowseResult, error) {
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
	b.url.Store(start)
	if frames != nil {
		defer b.cast(frames)()
	}
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

func browseDial(ctx context.Context, addr string) (string, http.Header, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", nil, fmt.Errorf("ARCHIVE_BROWSER: %w", err)
	}
	h := http.Header{}
	if host := u.Hostname(); u.Scheme == "https" && host != "localhost" && host != "127.0.0.1" {
		tok, err := gcpMetadata().get(ctx, "instance/service-accounts/default/identity?audience="+url.QueryEscape(u.Scheme+"://"+u.Host))
		if err != nil {
			return "", nil, fmt.Errorf("browser identity token: %w", err)
		}
		h.Set("Authorization", "Bearer "+tok)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	return u.String(), h, nil
}

func browseCall[P, R any](ctx context.Context, c cdp.Command[P, R], p P) (R, error) {
	return chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) (R, error) { return cdp.Call(ctx, t, c, p) })
}

func (b *browser) do(a chromedp.Action[chromedp.Void]) error {
	ctx, cancel := context.WithTimeout(b.ctx, 20*time.Second)
	defer cancel()
	_, err := chromedp.Run(ctx, a)
	return err
}

func (b *browser) cast(send func(frame, url string)) func() {
	ctx, cancel := context.WithCancel(b.ctx)
	events := chromedp.Events(ctx, page.ScreencastFrame)
	done := make(chan struct{})
	go func() {
		defer close(done)
		var last time.Time
		for ev, err := range events {
			if err != nil {
				return
			}
			browseCall(ctx, page.ScreencastFrameAck, page.ScreencastFrameAckParams{SessionID: ev.SessionID})
			if time.Since(last) >= 400*time.Millisecond {
				last = time.Now()
				u, _ := b.url.Load().(string)
				send(base64.StdEncoding.EncodeToString(ev.Data), u)
			}
		}
	}()
	q := int64(50)
	browseCall(ctx, page.StartScreencast, page.StartScreencastParams{Format: page.StartScreencastFormatJpeg, Quality: &q, MaxWidth: 640, MaxHeight: 400, EveryNthFrame: 2})
	return func() { cancel(); <-done }
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
    const text = (el.getAttribute('aria-label') || el.innerText || el.value || el.placeholder || el.title || el.alt || el.name || '').replace(/\s+/g, ' ').trim().slice(0, 80);
    let tag = el.tagName.toLowerCase();
    if (tag === 'input') tag += ' ' + (el.type || 'text');
    else if (tag === 'a') tag = 'link';
    els.push({ tag, text, x, y, w, h });
    if (els.length >= 150) break;
  }
  return { url: location.href, title: document.title, els };
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
	b.url.Store(v.URL)
	shot, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	if v.shot, err = browseMark(shot, v.Els); err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	return &v, nil
}

var browseColors = []color.RGBA{{220, 38, 38, 255}, {37, 99, 235, 255}, {22, 163, 74, 255}, {147, 51, 234, 255}, {234, 88, 12, 255}, {8, 145, 178, 255}}

func browseMark(shot []byte, els []browseEl) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(shot))
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, image.Point{}, draw.Src)
	for i, e := range els {
		c := image.NewUniform(browseColors[i%len(browseColors)])
		r := image.Rect(int(e.X), int(e.Y), int(e.X+e.W), int(e.Y+e.H)).Intersect(img.Bounds())
		for _, edge := range []image.Rectangle{{r.Min, image.Pt(r.Max.X, r.Min.Y+2)}, {image.Pt(r.Min.X, r.Max.Y-2), r.Max}, {r.Min, image.Pt(r.Min.X+2, r.Max.Y)}, {image.Pt(r.Max.X-2, r.Min.Y), r.Max}} {
			draw.Draw(img, edge, c, image.Point{}, draw.Src)
		}
		id := strconv.Itoa(i + 1)
		lx, ly := r.Min.X, r.Min.Y-14
		if ly < 0 {
			ly = r.Min.Y
		}
		draw.Draw(img, image.Rect(lx, ly, lx+7*len(id)+4, ly+14), c, image.Point{}, draw.Src)
		d := &font.Drawer{Dst: img, Src: image.White, Face: basicfont.Face7x13, Dot: fixed.P(lx+2, ly+11)}
		d.DrawString(id)
	}
	var out bytes.Buffer
	err = png.Encode(&out, img)
	return out.Bytes(), err
}

type jevAnswer struct {
	Choice     string  `json:"choice"`
	Confidence float64 `json:"confidence"`
	Noul       float64 `json:"noul"`
}

var browseActions = map[string]string{
	"click":        "Click one marked element: a link, button, tab, checkbox or menu item",
	"type":         "Type into one marked input, search box or text area",
	"scroll_down":  "Scroll down to see more of the page",
	"scroll_up":    "Scroll back up",
	"back":         "Go back to the previous page",
	"key_enter":    "Press Enter to submit what was just typed",
	"done":         "`goal` is achieved, or what it asks for is visible on `screen` now",
	"need_planner": "None of these fits: the page is blocked or confusing, or `goal` needs another website",
}

// next asks Jev for the next action and hands off to Claude when it needs text or Jev is unsure.
func (b *browser) next(ctx context.Context, v *browseView) (*browseAct, error) {
	if len(v.Els) == 0 && (v.URL == "" || v.URL == "about:blank") {
		return b.plan(ctx, v, "")
	}
	ref := "img-" + uuid.NewString()
	marks := map[string]string{"none": "No marked element fits"}
	for i, e := range v.Els {
		marks[strconv.Itoa(i+1)] = e.String()
	}
	a, err := b.ask(ctx, map[string]any{
		"model":  browseJev,
		"state":  map[string]any{"goal": b.goal, "page": v.Title + " " + v.URL, "history": b.history, "screen": ref},
		"assets": map[string]any{ref: map[string]string{"type": "image", "format": "png", "data_base64": base64.StdEncoding.EncodeToString(v.shot)}},
		"questions": map[string]any{
			"next":   map[string]any{"type": "choice", "instructions": "`screen` is a browser screenshot with numbered marks on the elements that can be used. Given `history`, what is the single best next action toward `goal`?", "criteria": browseActions},
			"target": map[string]any{"type": "choice", "instructions": "Which numbered mark in `screen` is the element to click or type into next, to make progress on `goal`?", "criteria": marks},
			"done":   map[string]any{"type": "noul", "instructions": "`goal` is achieved, or the answer it asks for is visible on `screen`."},
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
		hr.Header.Set("TypeSafe-Danger-Zone-Images", "true")
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

const browseSystem = `You drive a web browser for a script, to reach its goal. You see a screenshot of the page with numbered marks on the elements you can use, the list of those elements, and the page's text. History lists the actions already taken, oldest first, and the screenshot shows the page they led to. Pick the one next action with the act tool:
- click or type use a mark number; type clicks the field itself, replaces its text, and with enter: true submits it.
- goto opens an address, for a start or when this site is a dead end.
- done ends: achieved says whether the goal was reached, and answer gives what the goal asks for, taken from the page, short and exact; when it cannot be reached, say why.`

var browseTool = map[string]any{
	"name":        "act",
	"description": "The next browser action, or done with the answer.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":   map[string]any{"type": "string", "enum": []string{"click", "type", "scroll_down", "scroll_up", "back", "key_enter", "goto", "done"}},
			"mark":     map[string]any{"type": "integer", "description": "For click and type: the numbered mark"},
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
	text, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(document.body ? document.body.innerText : '').replace(/\n\s*\n/g, '\n').slice(0, 6000)`))
	var s strings.Builder
	fmt.Fprintf(&s, "Goal: %s\nNow on: %s %s\nHistory:\n", b.goal, v.Title, v.URL)
	for _, h := range b.history {
		s.WriteString("- " + h + "\n")
	}
	s.WriteString("Marked elements:\n")
	for i, e := range v.Els {
		fmt.Fprintf(&s, "%d %s\n", i+1, e)
	}
	s.WriteString("Page text:\n" + text + "\n" + hint)
	body, err := json.Marshal(map[string]any{
		"model":       chatModel,
		"max_tokens":  2048,
		"system":      browseSystem,
		"tools":       []any{browseTool},
		"tool_choice": map[string]any{"type": "tool", "name": "act", "disable_parallel_tool_use": true},
		"messages": []any{map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "image", "source": map[string]string{"type": "base64", "media_type": "image/png", "data": base64.StdEncoding.EncodeToString(v.shot)}},
			map[string]any{"type": "text", "text": s.String()},
		}}},
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
			err = fmt.Errorf("there is no mark %d", a.Mark)
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
