package archive

import (
	"bytes"
	"cmp"
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
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Intake struct {
	cfg    IntakeConfig
	save   func(ctx context.Context, r *Record, data map[string]any, prov []*Provenance) error
	spec   func() *Spec
	http   *http.Client
	cal    map[string]*intakeIsotonic
	labels map[string][]intakeLabel
	err    error
}

const intakeMinLabels = 10

func NewIntake(cfg IntakeConfig, spec func() *Spec, save func(ctx context.Context, r *Record, data map[string]any, prov []*Provenance) error) *Intake {
	if cfg.Samples <= 0 {
		cfg.Samples = 3
	}
	if cfg.Accept == 0 {
		cfg.Accept = 0.9
	}
	if cfg.Review == 0 {
		cfg.Review = 0.5
	}
	in := &Intake{cfg: cfg, save: save, spec: spec, http: &http.Client{Timeout: 5 * time.Minute}}
	if cfg.Labels != "" {
		in.labels, in.err = intakeLoadLabels(cfg.Labels)
		in.cal = intakeFitAll(in.labels)
	}
	return in
}

func (in *Intake) Ingest(ctx context.Context, raw *Raw, typ string) (*IntakeResult, error) {
	if in.err != nil {
		return nil, in.err
	}
	spec := in.spec()
	if spec == nil || spec.Types[typ] == nil {
		return nil, fmt.Errorf("intake: unknown type %q", typ)
	}

	var payload struct {
		Text  string     `json:"text"`
		File  *uuid.UUID `json:"file"`
		Pages []string   `json:"pages"`
	}
	if err := json.Unmarshal(raw.Payload, &payload); err != nil {
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

	res := &IntakeResult{RawID: raw.ID, Type: typ}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	complete, accepted := true, map[string]any{}
	for _, name := range names {
		value, original, score, backed := intakeScore(samples, name)
		conf := in.calibrate(typ, name, score)
		route := intakeRoute(conf, in.cfg.Accept, in.cfg.Review)
		if route == "accept" && !backed && in.cal[typ+"."+name] == nil {
			// neither token probabilities nor labels stand behind this score, so a person looks first
			route = "review"
		}
		if value == nil {
			route = "drop"
		}
		if route == "accept" {
			accepted[name] = value
		} else if fields[name].Required {
			complete = false
		}
		if spec.secretField(typ, name) {
			original = ""
		}
		res.Fields = append(res.Fields, FieldResult{Path: name, Value: value, Original: original, RawScore: score, Confidence: conf, Route: route})
	}

	saved := complete && len(accepted) > 0
	if saved {
		var prov []*Provenance
		for _, f := range res.Fields {
			if f.Route == "accept" {
				prov = append(prov, &Provenance{FieldPath: f.Path, RawIDs: []uuid.UUID{raw.ID}, FileID: payload.File,
					Page: intakePage(payload.Pages, f.Original, f.Value), Method: "intake:" + in.cfg.Model, Confidence: f.Confidence, Original: f.Original})
			}
		}
		rec := &Record{Type: typ, SpecVersion: spec.Version, Space: raw.Space}
		var ve *ValidationError
		switch err := in.save(ctx, rec, accepted, prov); {
		case errors.As(err, &ve):
			// the record as a whole fails a check no single field shows, such as a reference to an
			// object that does not exist: nothing is saved and a person decides
			saved = false
			for i := range res.Fields {
				if res.Fields[i].Route == "accept" {
					res.Fields[i].Route = "review"
				}
			}
		case err != nil:
			return nil, err
		default:
			res.RecordID = rec.ID
		}
	}
	for i, f := range res.Fields {
		if spec.secretField(typ, f.Path) {
			res.Fields[i].Value = "****"
			if v, ok := accepted[f.Path]; ok && saved {
				res.Fields[i].Value = v
			}
		}
	}
	return res, nil
}

func intakePage(pages []string, original string, v any) int {
	s, _ := v.(string)
	needle := cmp.Or(original, s)
	for i, p := range pages {
		if needle != "" && strings.Contains(p, needle) {
			return i + 1
		}
	}
	return 0
}

func (in *Intake) calibrate(typ, field string, score float64) float64 {
	if c := in.cal[typ+"."+field]; c != nil {
		return c.predict(score)
	}
	return score
}

func (in *Intake) CalibrationError() map[string]float64 {
	out := map[string]float64{}
	for key, ls := range in.labels {
		if len(ls) < 2*intakeMinLabels {
			continue
		}
		var conf []float64
		var correct []bool
		for fold := range 2 {
			var train, test []intakeLabel
			for i, l := range ls {
				if i%2 == fold {
					test = append(test, l)
				} else {
					train = append(train, l)
				}
			}
			iso := intakeFit(train)
			for _, l := range test {
				conf = append(conf, iso.predict(l.Score))
				correct = append(correct, l.Correct)
			}
		}
		out[key] = ExpectedCalibrationError(conf, correct)
	}
	return out
}

func ExpectedCalibrationError(conf []float64, correct []bool) float64 {
	const bins = 10
	var n, sum, hit [bins]float64
	for i, c := range conf {
		b := min(int(c*bins), bins-1)
		b = max(b, 0)
		n[b]++
		sum[b] += c
		if correct[i] {
			hit[b]++
		}
	}
	var ece float64
	for b := range bins {
		if n[b] > 0 {
			ece += n[b] / float64(len(conf)) * math.Abs(sum[b]/n[b]-hit[b]/n[b])
		}
	}
	return ece
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
	payload := map[string]any{
		"model":    cfg.Model,
		"logprobs": true,
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
	}
	// only when asked for: some models (Claude's, on Anthropic's OpenAI-compatible endpoint)
	// refuse a temperature, and the endpoint's own default samples well enough
	if cfg.Temperature != nil {
		payload["temperature"] = *cfg.Temperature
	}
	body, err := json.Marshal(payload)
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
	Value    any
	Original string
	Valid    bool
	Logprob  float64
	HasLP    bool
}

func intakeValidate(spec *Spec, typ string, c *intakeCompletion, h Hints) intakeSample {
	s := intakeSample{}
	var in map[string]any
	if json.Unmarshal([]byte(c.Content), &in) != nil {
		return s
	}
	out, originals, err := spec.Normalize(typ, in, h)
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
	if len(bad) > 0 {
		// one bad field voids Normalize's whole result, so the good ones are normalized one by one
		// rather than kept as the model wrote them
		out, originals = map[string]any{}, map[string]string{}
		fields, _ := spec.Fields(typ)
		for k, raw := range in {
			if f := fields[k]; f != nil && !bad[k] {
				if nv, err := spec.NormalizeField(f, raw, h); err != nil {
					bad[k] = true
				} else {
					out[k] = nv
					if !reflect.DeepEqual(raw, nv) {
						originals[k] = specString(raw)
					}
				}
			}
		}
	}
	for k, raw := range in {
		v := intakeValue{Value: raw, Valid: !bad[k]}
		if nv, ok := out[k]; ok && v.Valid {
			v.Value = nv
		}
		v.Original = originals[k]
		v.Logprob, v.HasLP = c.Logprobs[k]
		if v.Value != nil {
			s[k] = v
		}
	}
	return s
}

// intakeScore picks the value most samples agree on and scores it by agreement, validity and the
// model's own token probabilities. backed says whether those probabilities were there: without
// them (Ollama and other servers that ignore logprobs) agreement alone can reach 1.0, which says
// nothing about a value the model gets wrong the same way every time.
func intakeScore(samples []intakeSample, field string) (value any, original string, score float64, backed bool) {
	if len(samples) == 0 {
		return nil, "", 0, false
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
		return nil, "", 0, false
	}
	lp := 1.0
	if best.lpN > 0 {
		lp = math.Exp(best.lpSum / float64(best.lpN))
	}
	score = float64(best.n) / float64(len(samples)) * float64(best.valid) / float64(best.n) * lp
	return best.v.Value, best.v.Original, score, best.lpN > 0
}

type intakeLabel struct {
	Type    string  `json:"type"`
	Field   string  `json:"field"`
	Score   float64 `json:"score"`
	Correct bool    `json:"correct"`
}

func intakeLoadLabels(path string) (map[string][]intakeLabel, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("intake labels: %w", err)
	}
	var ls []intakeLabel
	if err := json.Unmarshal(b, &ls); err != nil {
		return nil, fmt.Errorf("intake labels %s: %w", path, err)
	}
	out := map[string][]intakeLabel{}
	for _, l := range ls {
		out[l.Type+"."+l.Field] = append(out[l.Type+"."+l.Field], l)
	}
	return out, nil
}

func intakeFitAll(labels map[string][]intakeLabel) map[string]*intakeIsotonic {
	out := map[string]*intakeIsotonic{}
	for key, ls := range labels {
		if len(ls) >= intakeMinLabels {
			out[key] = intakeFit(ls)
		}
	}
	return out
}

type intakeIsotonic struct {
	x, y []float64
}

func intakeFit(ls []intakeLabel) *intakeIsotonic {
	ls = append([]intakeLabel(nil), ls...)
	sort.Slice(ls, func(i, j int) bool { return ls[i].Score < ls[j].Score })
	var x, sum, w []float64
	for _, l := range ls {
		y := 0.0
		if l.Correct {
			y = 1
		}
		x, sum, w = append(x, l.Score), append(sum, y), append(w, 1)

		for n := len(w); n > 1 && sum[n-1]/w[n-1] <= sum[n-2]/w[n-2]; n-- {
			sum[n-2] += sum[n-1]
			w[n-2] += w[n-1]
			x, sum, w = x[:n-1], sum[:n-1], w[:n-1]
		}
	}
	iso := &intakeIsotonic{x: x}
	for i := range sum {
		iso.y = append(iso.y, sum[i]/w[i])
	}
	return iso
}

func (c *intakeIsotonic) predict(score float64) float64 {
	i := sort.Search(len(c.x), func(i int) bool { return c.x[i] > score }) - 1
	return c.y[max(i, 0)]
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

func ScanFolder(ctx context.Context, st *pgStore, dir string, space uuid.UUID) ([]*Raw, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var out []*Raw
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
		mt := folderMIME(path, b)
		payload, err := json.Marshal(map[string]any{
			"path": path, "name": d.Name(), "mime": mt, "size": len(b),
			"text": folderText(ctx, path, mt, b),
		})
		if err != nil {
			return err
		}
		sum := sha256.Sum256(payload)
		r := &Raw{ID: uuid.Must(uuid.NewV7()), Source: "folder:" + abs, ExternalID: filepath.ToSlash(rel),
			Hash: hex.EncodeToString(sum[:]), FetchedAt: time.Now().UTC(), Payload: payload, Space: space}
		inserted, err := st.PutRaw(ctx, r)
		if err != nil {
			return err
		}
		if inserted {
			out = append(out, r)
		}
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
