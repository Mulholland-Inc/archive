package main

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/jackc/pgx/v5"
)

type Plan struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Price        string `json:"price"`
	Runs         *int64 `json:"runs"`
	WorkflowRuns *int64 `json:"workflow_runs"`
	AITokens     *int64 `json:"ai_tokens"`
	StoredBytes  *int64 `json:"stored_bytes"`
	Concurrent   *int32 `json:"concurrent"`
}

const (
	stripeTolerance = 5 * time.Minute
	stripeMaxBody   = 1 << 20
)

var planIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func billingOn() bool { return os.Getenv("ARCHIVE_STRIPE_KEY") != "" }

func planOut(p generated.ArchivePlan) *Plan {
	return &Plan{ID: p.ID, Name: p.Name, Price: p.Price, Runs: p.Runs, WorkflowRuns: p.WorkflowRuns, AITokens: p.AiTokens, StoredBytes: p.StoredBytes, Concurrent: p.Concurrent}
}

func (h *Hub) Plans(ctx context.Context) ([]*Plan, error) {
	if ActorFrom(ctx) == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	rows, err := h.q().Plans(ctx)
	out := []*Plan{}
	for _, r := range rows {
		out = append(out, planOut(r))
	}
	return out, err
}

func (h *Hub) SetPlan(ctx context.Context, p Plan) (*Plan, error) {
	if err := h.admin(ctx); err != nil {
		return nil, err
	}
	ve := &ValidationError{Type: "plan"}
	if !planIDRe.MatchString(p.ID) {
		ve.Errors = append(ve.Errors, FieldError{Path: "id", Message: "want 1-32 lowercase letters, digits, _ or -"})
	}
	if p.Name = strings.TrimSpace(p.Name); p.Name == "" {
		p.Name = p.ID
	}
	for k, v := range map[string]*int64{"runs": p.Runs, "workflow_runs": p.WorkflowRuns, "ai_tokens": p.AITokens, "stored_bytes": p.StoredBytes} {
		if v != nil && *v < 0 {
			ve.Errors = append(ve.Errors, FieldError{Path: k, Message: "want 0 or more, or null for no limit"})
		}
	}
	if p.Concurrent != nil && *p.Concurrent < 1 {
		ve.Errors = append(ve.Errors, FieldError{Path: "concurrent", Message: "want 1 or more, or null for the default"})
	}
	if len(ve.Errors) > 0 {
		return nil, ve
	}
	r, err := h.q().SavePlan(ctx, generated.SavePlanParams{ID: p.ID, Name: p.Name, Price: strings.TrimSpace(p.Price), Runs: p.Runs,
		WorkflowRuns: p.WorkflowRuns, AiTokens: p.AITokens, StoredBytes: p.StoredBytes, Concurrent: p.Concurrent})
	if err != nil {
		return nil, err
	}
	return planOut(r), nil
}

func (a *Archive) limit(ctx context.Context, what string) error {
	if !billingOn() {
		return nil
	}
	l, err := a.hub.q().WorkspaceLimits(ctx, a.ws.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var max *int64
	var used int64
	switch what {
	case "runs":
		max, used = l.Runs, l.UsedRuns
	case "workflow runs":
		max, used = l.WorkflowRuns, l.UsedWorkflowRuns
	case "AI tokens":
		max, used = l.AiTokens, l.UsedAiTokens
	case "stored bytes":
		max, used = l.StoredBytes, l.UsedStoredBytes
	}
	if max == nil || used < *max {
		return nil
	}
	return fmt.Errorf("this workspace used its %d %s for this month; a bigger plan has more: %w", *max, what, ErrLimit)
}

func stripeDo(ctx context.Context, method, path string, form url.Values, out any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(cmp.Or(os.Getenv("ARCHIVE_STRIPE_URL"), "https://api.stripe.com"), "/")+path, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(os.Getenv("ARCHIVE_STRIPE_KEY"), "")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("stripe: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, stripeMaxBody))
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(b, &e)
		return fmt.Errorf("stripe: %s", cmp.Or(e.Error.Message, resp.Status))
	}
	return json.Unmarshal(b, out)
}

func (a *Archive) Checkout(ctx context.Context, planID string) (string, error) {
	ac, err := a.workspaceAdmin(ctx)
	if err != nil {
		return "", err
	}
	if !billingOn() {
		return "", fmt.Errorf("billing is off on this Archive: %w", ErrNotFound)
	}
	p, err := a.hub.q().PlanByID(ctx, planID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && p.Price == "" {
		return "", fmt.Errorf("plan %q has no price: %w", planID, ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	row, err := a.hub.q().WorkspaceByShort(ctx, a.ws.Short)
	if err != nil {
		return "", err
	}
	f := url.Values{
		"mode":                                   {"subscription"},
		"line_items[0][price]":                   {p.Price},
		"line_items[0][quantity]":                {"1"},
		"client_reference_id":                    {row.Short},
		"metadata[workspace]":                    {row.Short},
		"subscription_data[metadata][workspace]": {row.Short},
		"success_url":                            {a.hub.app + "/?billing=done"},
		"cancel_url":                             {a.hub.app + "/"},
	}
	if row.Customer != "" {
		f.Set("customer", row.Customer)
	} else {
		f.Set("customer_email", ac.Email)
	}
	var s struct {
		URL string `json:"url"`
	}
	if err := stripeDo(ctx, "POST", "/v1/checkout/sessions", f, &s); err != nil {
		return "", err
	}
	return s.URL, nil
}

type stripeSub struct {
	ID       string            `json:"id"`
	Customer string            `json:"customer"`
	Status   string            `json:"status"`
	Metadata map[string]string `json:"metadata"`
	Items    struct {
		Data []struct {
			Price struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

func stripeVerify(header string, body []byte, secret string, now time.Time) bool {
	var ts int64
	var sigs []string
	for _, kv := range strings.Split(header, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(kv), "=")
		switch k {
		case "t":
			ts, _ = strconv.ParseInt(v, 10, 64)
		case "v1":
			sigs = append(sigs, v)
		}
	}
	if secret == "" || ts == 0 || now.Sub(time.Unix(ts, 0)).Abs() > stripeTolerance {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(m, "%d.%s", ts, body)
	want := hex.EncodeToString(m.Sum(nil))
	for _, s := range sigs {
		if hmac.Equal([]byte(s), []byte(want)) {
			return true
		}
	}
	return false
}

func (h *Hub) stripe(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, stripeMaxBody))
	if err != nil {
		httpWrite(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "the body is too large"}, nil)
		return
	}
	if !billingOn() || !stripeVerify(r.Header.Get("Stripe-Signature"), b, os.Getenv("ARCHIVE_STRIPE_WEBHOOK_SECRET"), time.Now()) {
		httpWrite(w, http.StatusBadRequest, map[string]string{"error": "the Stripe-Signature header does not match"}, nil)
		return
	}
	var ev struct {
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &ev); err != nil {
		httpWrite(w, 0, nil, fmt.Errorf("body: %v: %w", err, httpErrBadRequest))
		return
	}
	ctx := r.Context()
	var sub stripeSub
	short := ""
	switch ev.Type {
	case "checkout.session.completed":
		var s struct {
			Reference    string `json:"client_reference_id"`
			Subscription string `json:"subscription"`
		}
		json.Unmarshal(ev.Data.Object, &s)
		if s.Subscription == "" {
			break
		}
		if err = stripeDo(ctx, "GET", "/v1/subscriptions/"+url.PathEscape(s.Subscription), nil, &sub); err == nil {
			short = cmp.Or(s.Reference, sub.Metadata["workspace"])
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		json.Unmarshal(ev.Data.Object, &sub)
		if ev.Type == "customer.subscription.deleted" {
			sub.Status = "canceled"
		}
		short = sub.Metadata["workspace"]
		if short == "" && sub.ID != "" {
			if row, err := h.q().WorkspaceBySubscription(ctx, sub.ID); err == nil {
				short = row.Short
			}
		}
	}
	if err == nil && short != "" {
		err = h.subscribe(ctx, short, sub)
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	httpWrite(w, http.StatusOK, map[string]bool{"received": true}, nil)
}

func (h *Hub) subscribe(ctx context.Context, short string, sub stripeSub) error {
	p := generated.SetWorkspacePlanParams{Short: short, Customer: sub.Customer, Subscription: sub.ID}
	switch sub.Status {
	case "active", "trialing", "past_due":
		if len(sub.Items.Data) == 0 {
			return nil
		}
		plan, err := h.q().PlanByPrice(ctx, sub.Items.Data[0].Price.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no plan has the Stripe price %s: %w", sub.Items.Data[0].Price.ID, ErrNotFound)
		}
		if err != nil {
			return err
		}
		p.Plan = &plan.ID
	case "canceled", "unpaid", "incomplete_expired":
		p.Plan, p.Subscription = h.defaultPlan(ctx), ""
	default:
		return nil
	}
	return h.q().SetWorkspacePlan(ctx, p)
}
