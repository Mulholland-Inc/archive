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
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
)

const (
	modelTimeout  = 10 * time.Minute
	modelCall     = 2 * time.Minute
	modelMaxFile  = 20 << 20
	modelMaxAll   = 40 << 20
	modelMaxN     = 10
	modelTokens   = 16000
	modelParallel = 8
	modelMaxList  = 64
	modelMaxHeld  = 200 << 20
)

type ModelResult struct {
	Text      string          `json:"text"`
	JSON      json.RawMessage `json:"json,omitempty"`
	TokensIn  int             `json:"tokens_in"`
	TokensOut int             `json:"tokens_out"`
}

type modelFile struct {
	ID   uuid.UUID `json:"id"`
	Page int       `json:"page"`
}

type modelReq struct {
	Model  string          `json:"model"`
	System string          `json:"system"`
	Prompt string          `json:"prompt"`
	Files  []modelFile     `json:"files"`
	Schema json.RawMessage `json:"schema"`
	Effort string          `json:"effort"`
}

// modelPart is a file as a model sees it: one PDF, one page of one, or an image.
type modelPart struct {
	mime string
	data []byte
}

type modelProvider = func(ctx context.Context, req *modelReq, parts []modelPart) (*ModelResult, error)

var modelMIMEs = map[string]bool{"application/pdf": true, "image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true}

// modelProviders maps a model name's prefix to the provider that serves it.
var modelProviders = map[string]modelProvider{
	"gemini-": modelGemini,
	"claude-": modelClaude,
}

func init() { api.DisableConfigDir() }

func (r *runner) modelSDK(o *goja.Object) {
	o.Set("model", func(call goja.FunctionCall) goja.Value { return r.promise(r.model(call)) })
}

// model takes one request or a list. A list runs at once, modelParallel at a time, and fails on its first error.
func (r *runner) model(call goja.FunctionCall) (any, error) {
	reqs, list := []modelReq{{}}, false
	if _, list = call.Argument(0).Export().([]any); list {
		reqs = nil
		if err := r.arg(call, 0, "model", "reqs", &reqs); err != nil {
			return nil, err
		}
	} else if err := r.arg(call, 0, "model", "req", &reqs[0]); err != nil {
		return nil, err
	}
	providers, files := make([]modelProvider, len(reqs)), make([][]modelPart, len(reqs))
	for i, req := range reqs {
		if req.Model == "" || req.Prompt == "" {
			return nil, fmt.Errorf("archive.model: model and prompt are required: %w", httpErrBadRequest)
		}
		if strings.Trim(req.Model, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-") != "" {
			return nil, fmt.Errorf("archive.model: %q is not a model name: %w", req.Model, httpErrBadRequest)
		}
		if req.Effort != "" && req.Effort != "none" {
			return nil, fmt.Errorf("archive.model: effort must be \"none\" or left out: %w", httpErrBadRequest)
		}
		for prefix, p := range modelProviders {
			if strings.HasPrefix(req.Model, prefix) {
				providers[i] = p
			}
		}
		if providers[i] == nil {
			return nil, fmt.Errorf("archive.model: %q is not a model this server can call; use a gemini- or claude- model: %w", req.Model, httpErrBadRequest)
		}
	}
	if len(reqs) > modelMaxList {
		return nil, fmt.Errorf("archive.model: %d requests is more than the %d one call takes: %w", len(reqs), modelMaxList, httpErrBadRequest)
	}
	// Files open through the run's transaction, which only this goroutine may use; the whole list is held in
	// memory while it runs, so it is bounded as a whole too.
	held := 0
	for i, req := range reqs {
		var err error
		if files[i], err = r.modelFiles(req.Files); err != nil {
			return nil, fmt.Errorf("archive.model: %w", err)
		}
		for _, part := range files[i] {
			if held += len(part.data); held > modelMaxHeld {
				return nil, fmt.Errorf("archive.model: the list's files run over the %d MB one call holds: %w", modelMaxHeld>>20, httpErrBadRequest)
			}
		}
	}
	resume, err := r.clock.pause(&r.clock.model, modelTimeout, "called models")
	if err != nil {
		return nil, fmt.Errorf("archive.model: %w", err)
	}
	defer resume()
	ctx, cancel := context.WithTimeout(r.ctx, min(modelCall, r.clock.model))
	defer cancel()
	results := make([]*ModelResult, len(reqs))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(modelParallel)
	// The budget pays for every call's time, not the list's wall time, so a list of eight buys no more than eight calls.
	var spent atomic.Int64
	start := time.Now()
	for i := range reqs {
		g.Go(func() (err error) {
			t := time.Now()
			results[i], err = providers[i](ctx, &reqs[i], files[i])
			spent.Add(int64(time.Since(t)))
			return err
		})
	}
	err = g.Wait()
	r.clock.model -= max(0, time.Duration(spent.Load())-time.Since(start))
	if err != nil {
		return nil, fmt.Errorf("archive.model: %w", err)
	}
	if !list {
		return results[0], nil
	}
	return results, nil
}

// modelFiles loads each file the caller may read, whole or one PDF page, within the size and type limits.
func (r *runner) modelFiles(files []modelFile) ([]modelPart, error) {
	if len(files) > modelMaxN {
		return nil, fmt.Errorf("%d files; a model reads at most %d a call: %w", len(files), modelMaxN, httpErrBadRequest)
	}
	parts, total := make([]modelPart, 0, len(files)), 0
	for _, mf := range files {
		f, rc, err := r.a.OpenFile(r.ctx, mf.ID)
		if err != nil {
			return nil, err
		}
		if !modelMIMEs[f.MIME] {
			rc.Close()
			return nil, fmt.Errorf("file %s is %s; a model reads PDF, PNG, JPEG, WebP and GIF files: %w", mf.ID, f.MIME, httpErrBadRequest)
		}
		if f.Size > modelMaxFile {
			rc.Close()
			return nil, fmt.Errorf("file %s is %d MB, over the %d MB a model reads: %w", mf.ID, f.Size>>20, modelMaxFile>>20, httpErrBadRequest)
		}
		data, err := io.ReadAll(io.LimitReader(rc, modelMaxFile+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if total += len(data); len(data) > modelMaxFile || total > modelMaxAll {
			return nil, fmt.Errorf("the files run over the %d MB a model reads in one call: %w", modelMaxAll>>20, httpErrBadRequest)
		}
		r.out.change(runChange{f.ID, f.Name, "read"})
		if mf.Page != 0 {
			if f.MIME != "application/pdf" {
				return nil, fmt.Errorf("file %s: page is for PDF files only: %w", mf.ID, httpErrBadRequest)
			}
			if data, err = modelPage(data, mf.Page); err != nil {
				return nil, fmt.Errorf("file %s: %w", mf.ID, err)
			}
		}
		parts = append(parts, modelPart{f.MIME, data})
	}
	return parts, nil
}

// modelPage cuts page (1-based) out of a PDF as a PDF of its own.
func modelPage(data []byte, page int) ([]byte, error) {
	ctx, err := api.ReadValidateAndOptimize(bytes.NewReader(data), model.NewDefaultConfiguration())
	if err != nil {
		return nil, fmt.Errorf("not a readable PDF: %v", err)
	}
	if page < 1 || page > ctx.PageCount {
		return nil, fmt.Errorf("page %d is out of range; the PDF has %d pages: %w", page, ctx.PageCount, httpErrBadRequest)
	}
	one, err := pdfcpu.ExtractPages(ctx, []int{page}, false)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := api.WriteContext(one, &b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func modelKey(env string) (string, error) {
	key := strings.TrimSpace(os.Getenv(env))
	if key == "" {
		return "", fmt.Errorf("this model is not configured on this server; it needs %s", env)
	}
	return key, nil
}

func modelDecode(resp *http.Response, v any) error {
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("model: reading the answer: %v", err)
	}
	return nil
}

func modelGemini(ctx context.Context, req *modelReq, parts []modelPart) (*ModelResult, error) {
	url, header, err := geminiEndpoint(ctx, req.Model)
	if err != nil {
		return nil, err
	}
	content := []any{map[string]any{"text": req.Prompt}}
	for _, p := range parts {
		content = append(content, map[string]any{"inline_data": map[string]any{"mime_type": p.mime, "data": base64.StdEncoding.EncodeToString(p.data)}})
	}
	gen := map[string]any{}
	if req.Schema != nil {
		gen["responseMimeType"], gen["responseJsonSchema"] = "application/json", req.Schema
	}
	if req.Effort == "none" {
		gen["thinkingConfig"] = map[string]any{"thinkingBudget": 0}
	}
	body := map[string]any{"contents": []any{map[string]any{"role": "user", "parts": content}}, "generationConfig": gen}
	if req.System != "" {
		body["system_instruction"] = map[string]any{"parts": []any{map[string]any{"text": req.System}}}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := modelPost(ctx, url, header, b)
	if err != nil {
		return nil, err
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Usage struct {
			In  int `json:"promptTokenCount"`
			Out int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := modelDecode(resp, &out); err != nil {
		return nil, err
	}
	if len(out.Candidates) == 0 {
		return nil, errors.New("model: no answer")
	}
	var text strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		text.WriteString(p.Text)
	}
	res := &ModelResult{Text: text.String(), TokensIn: out.Usage.In, TokensOut: out.Usage.Out}
	if req.Schema != nil {
		if !json.Valid([]byte(res.Text)) {
			return nil, errors.New("model: the answer is not the JSON the schema asked for")
		}
		res.JSON = json.RawMessage(res.Text)
	}
	return res, nil
}

// geminiEndpoint is the Gemini API with GEMINI_API_KEY, else Vertex in GOOGLE_CLOUD_PROJECT as the machine's own
// Google credentials (gcloud's on a laptop, the service account on Cloud Run), in VERTEX_LOCATION, us by default.
func geminiEndpoint(ctx context.Context, model string) (string, http.Header, error) {
	if key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); key != "" {
		return "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent", http.Header{"x-goog-api-key": {key}}, nil
	}
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "" {
		return "", nil, errors.New("this model is not configured on this server; it needs GEMINI_API_KEY, or GOOGLE_CLOUD_PROJECT for Vertex")
	}
	geminiCreds.Do(func() {
		geminiCreds.c, geminiCreds.err = google.FindDefaultCredentials(context.WithoutCancel(ctx), "https://www.googleapis.com/auth/cloud-platform")
	})
	if geminiCreds.err != nil {
		return "", nil, fmt.Errorf("vertex: %w", geminiCreds.err)
	}
	tok, err := geminiCreds.c.TokenSource.Token()
	if err != nil {
		return "", nil, fmt.Errorf("vertex: %w", err)
	}
	loc := cmp.Or(os.Getenv("VERTEX_LOCATION"), "us")
	host := loc + "-aiplatform.googleapis.com"
	switch loc {
	case "global":
		host = "aiplatform.googleapis.com"
	case "us", "eu":
		host = "aiplatform." + loc + ".rep.googleapis.com"
	}
	return fmt.Sprintf("https://%s/v1beta1/projects/%s/locations/%s/publishers/google/models/%s:generateContent", host, project, loc, model),
		http.Header{"Authorization": {"Bearer " + tok.AccessToken}}, nil
}

var geminiCreds struct {
	sync.Once
	c   *google.Credentials
	err error
}

// modelBlock is a file as an Anthropic content block: a document for a PDF, an image otherwise.
func modelBlock(p modelPart) agentBlock {
	kind := "image"
	if p.mime == "application/pdf" {
		kind = "document"
	}
	return agentBlock{"type": kind, "source": map[string]any{"type": "base64", "media_type": p.mime, "data": base64.StdEncoding.EncodeToString(p.data)}}
}

func modelClaude(ctx context.Context, req *modelReq, parts []modelPart) (*ModelResult, error) {
	key, err := modelKey("ANTHROPIC_API_KEY")
	if err != nil {
		return nil, err
	}
	content := []agentBlock{{"type": "text", "text": req.Prompt}}
	for _, p := range parts {
		content = append(content, modelBlock(p))
	}
	body := map[string]any{"model": req.Model, "max_tokens": modelTokens, "messages": []agentMessage{{Role: "user", Content: content}}}
	if req.System != "" {
		body["system"] = req.System
	}
	if req.Schema != nil {
		body["tools"] = []any{map[string]any{"name": "answer", "input_schema": req.Schema}}
		body["tool_choice"] = map[string]any{"type": "tool", "name": "answer"}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := agentPost(ctx, key, b)
	if err != nil {
		return nil, err
	}
	var out struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		Usage struct {
			In  int `json:"input_tokens"`
			Out int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := modelDecode(resp, &out); err != nil {
		return nil, err
	}
	res := &ModelResult{TokensIn: out.Usage.In, TokensOut: out.Usage.Out}
	var text strings.Builder
	for _, c := range out.Content {
		switch c.Type {
		case "text":
			text.WriteString(c.Text)
		case "tool_use":
			res.JSON = c.Input
		}
	}
	res.Text = text.String()
	if req.Schema != nil {
		if res.JSON == nil {
			return nil, errors.New("model: the answer is not the JSON the schema asked for")
		}
		res.Text = string(res.JSON)
	}
	return res, nil
}
