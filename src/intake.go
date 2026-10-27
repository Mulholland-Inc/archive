package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Intake struct {
	cfg  IntakeConfig
	save func(ctx context.Context, e *Entity, data map[string]any) error
	spec func() *Spec
	http *http.Client
}

const intakeTemperature = 0.7

func NewIntake(cfg IntakeConfig, spec func() *Spec, save func(ctx context.Context, e *Entity, data map[string]any) error) *Intake {
	if cfg.Samples <= 0 {
		cfg.Samples = 3
	}
	if cfg.Accept == 0 {
		cfg.Accept = 0.9
	}
	if cfg.Review == 0 {
		cfg.Review = 0.5
	}
	return &Intake{cfg: cfg, save: save, spec: spec, http: &http.Client{Timeout: 5 * time.Minute}}
}

func (in *Intake) Ingest(ctx context.Context, it *Item, typ string) (*IntakeResult, error) {
	spec := in.spec()
	if spec == nil || spec.Types[typ] == nil {
		return nil, fmt.Errorf("intake: unknown type %q", typ)
	}
	var payload struct {
		Text string     `json:"text"`
		File *uuid.UUID `json:"file"`
	}
	if err := json.Unmarshal(it.Payload, &payload); err != nil {
		return nil, fmt.Errorf("intake: payload: %w", err)
	}
	fields, err := spec.Fields(typ)
	if err != nil {
		return nil, err
	}
	schema, err := spec.JSONSchema(typ)
	if err != nil {
		return nil, err
	}

	var samples []intakeSample
	for range in.cfg.Samples {
		c, err := intakeChat(ctx, in.http, in.cfg, typ, schema, intakePrompt(typ, spec.Types[typ].Description), payload.Text)
		if err != nil {
			return nil, err
		}
		samples = append(samples, intakeValidate(spec, typ, c, in.cfg.Hints))
	}

	res := &IntakeResult{Source: it.Source, ExternalID: it.ExternalID, Type: typ, Confidence: 1}
	complete, data := true, map[string]any{}
	for _, name := range specKeys(fields) {
		f := fields[name]
		value, score := intakeScore(samples, name)
		if payload.File != nil && f.Type == specReference && !f.List && slices.Contains(spec.Subtypes("file"), f.To) {
			value, score = payload.File.String(), 1
		}
		route := intakeRoute(score, in.cfg.Accept, in.cfg.Review)
		if value == nil {
			route = "drop"
		}
		if route != "drop" {
			data[name] = value
			res.Confidence = min(res.Confidence, score)
		} else if f.Required {
			complete = false
		}
		res.Fields = append(res.Fields, FieldResult{Path: name, Value: value, Confidence: score, Route: route})
	}
	if len(data) == 0 {
		res.Confidence = 0
	} else {
		res.Proposed = !complete || res.Confidence < in.cfg.Accept
		e := &Entity{Type: typ, Space: it.Space, Proposed: res.Proposed}
		if err := in.save(ctx, e, data); err != nil {
			return nil, err
		}
		res.EntityID = e.ID
	}
	for i, f := range res.Fields {
		if spec.secretField(typ, f.Path) {
			res.Fields[i].Value = "****"
			if v, ok := data[f.Path]; ok {
				res.Fields[i].Value = v
			}
		}
	}
	return res, nil
}

type intakeCompletion struct {
	Content  string
	Logprobs map[string]float64
}

var intakeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func intakePrompt(typ, description string) string {
	p := "Extract one " + typ + " record from the document the user sends. Reply only with JSON matching the schema. Copy values as they appear; leave out fields the document does not contain."
	if description != "" {
		p += "\n\n" + typ + ": " + description
	}
	return p
}

func intakeChat(ctx context.Context, hc *http.Client, cfg IntakeConfig, typ string, schema map[string]any, system, text string) (*intakeCompletion, error) {
	body, err := json.Marshal(map[string]any{
		"model":       cfg.Model,
		"temperature": intakeTemperature,
		"logprobs":    true,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": text},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   intakeNameRe.ReplaceAllString(typ, "_"),
				"schema": schema,
				"strict": true,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(cfg.Endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("intake: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Logprobs *struct {
				Content []intakeToken `json:"content"`
			} `json:"logprobs"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("intake: %s: %w", resp.Status, err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("intake: %s: %s", resp.Status, out.Error.Message)
	}
	if resp.StatusCode != http.StatusOK || len(out.Choices) == 0 {
		return nil, fmt.Errorf("intake: %s: no choices", resp.Status)
	}
	c := &intakeCompletion{Content: out.Choices[0].Message.Content}
	if lp := out.Choices[0].Logprobs; lp != nil {
		c.Logprobs = intakeValueLogprobs(c.Content, lp.Content)
	}
	return c, nil
}

type intakeToken struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

func intakeValueLogprobs(content string, toks []intakeToken) map[string]float64 {
	var sb strings.Builder
	starts := make([]int, len(toks))
	for i, t := range toks {
		starts[i] = sb.Len()
		sb.WriteString(t.Token)
	}
	if sb.String() != content {
		return nil
	}
	out := map[string]float64{}
	dec := json.NewDecoder(strings.NewReader(content))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return out
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return out
		}
		end := int(dec.InputOffset())
		start := end - len(v)
		var sum float64
		var n int
		for i, t := range toks {
			if starts[i] < end && starts[i]+len(t.Token) > start {
				sum += t.Logprob
				n++
			}
		}
		if k, ok := key.(string); ok && n > 0 {
			out[k] = sum / float64(n)
		}
	}
	return out
}

type intakeSample map[string]intakeValue

type intakeValue struct {
	Value   any
	Valid   bool
	Logprob float64
	HasLP   bool
}

func intakeValidate(spec *Spec, typ string, c *intakeCompletion, h Hints) intakeSample {
	s := intakeSample{}
	var in map[string]any
	if json.Unmarshal([]byte(c.Content), &in) != nil {
		return s
	}
	out, err := spec.Normalize(typ, in, h)
	bad := map[string]bool{}
	var ve *ValidationError
	if errors.As(err, &ve) {
		for _, fe := range ve.Errors {
			if parts := strings.FieldsFunc(fe.Path, func(r rune) bool { return r == '/' || r == '.' || r == '[' }); len(parts) > 0 {
				bad[parts[0]] = true
			}
		}
	} else if err != nil {
		return s
	}
	for k, raw := range in {
		v := intakeValue{Value: raw, Valid: !bad[k]}
		if nv, ok := out[k]; ok && v.Valid {
			v.Value = nv
		}
		v.Logprob, v.HasLP = c.Logprobs[k]
		if v.Value != nil {
			s[k] = v
		}
	}
	return s
}

func intakeScore(samples []intakeSample, field string) (value any, score float64) {
	if len(samples) == 0 {
		return nil, 0
	}
	type group struct {
		v        intakeValue
		n, valid int
		lpSum    float64
		lpN      int
	}
	groups := map[string]*group{}
	var best *group
	for _, s := range samples {
		v, ok := s[field]
		if !ok {
			continue
		}
		b, _ := json.Marshal(v.Value)
		g := groups[string(b)]
		if g == nil {
			g = &group{v: v}
			groups[string(b)] = g
		}
		g.n++
		if v.Valid {
			g.valid++
		}
		if v.HasLP {
			g.lpSum += v.Logprob
			g.lpN++
		}
		if best == nil || g.n > best.n || (g.n == best.n && g.valid > best.valid) {
			best = g
		}
	}
	if best == nil {
		return nil, 0
	}
	lp := 1.0
	if best.lpN > 0 {
		lp = math.Exp(best.lpSum / float64(best.lpN))
	}
	score = float64(best.n) / float64(len(samples)) * float64(best.valid) / float64(best.n) * lp
	return best.v.Value, score
}

func intakeRoute(conf, accept, review float64) string {
	switch {
	case conf >= accept:
		return "accept"
	case conf >= review:
		return "review"
	}
	return "drop"
}

var folderTextExt = map[string]bool{".txt": true, ".md": true, ".csv": true, ".json": true, ".html": true, ".htm": true, ".eml": true}

func ScanFolder(ctx context.Context, dir string, space uuid.UUID, seen map[string]string) ([]*Item, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var out []*Item
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != abs && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		sum := sha256.Sum256(b)
		hash := hex.EncodeToString(sum[:])
		if seen != nil && seen[rel] == hash {
			return nil
		}
		mt := folderMIME(path, b)
		payload, err := json.Marshal(map[string]any{
			"path": path, "name": d.Name(), "mime": mt, "size": len(b),
			"text": folderText(ctx, path, mt, b),
		})
		if err != nil {
			return err
		}
		if seen != nil {
			seen[rel] = hash
		}
		out = append(out, &Item{Source: "folder:" + abs, ExternalID: rel, Payload: payload, Space: space})
		return nil
	})
	return out, err
}

func folderMIME(path string, b []byte) string {
	mt, _, _ := mime.ParseMediaType(http.DetectContentType(b))
	if mt == "application/octet-stream" || mt == "text/plain" {
		if ext, _, err := mime.ParseMediaType(mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))); err == nil {
			return ext
		}
	}
	return mt
}

func folderText(ctx context.Context, path, mt string, b []byte) string {
	var cmd []string
	switch {
	case folderTextExt[strings.ToLower(filepath.Ext(path))]:
		return strings.ToValidUTF8(string(b), "�")
	case mt == "application/pdf":
		cmd = []string{"pdftotext", "-q", path, "-"}
	case strings.HasPrefix(mt, "image/"):
		cmd = []string{"tesseract", path, "-"}
	default:
		return ""
	}
	if _, err := exec.LookPath(cmd[0]); err != nil {
		return ""
	}
	o, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ToValidUTF8(string(o), "�"))
}
