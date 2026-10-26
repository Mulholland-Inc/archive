package archive

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type evidenceTransport func(*http.Request) (*http.Response, error)

func (f evidenceTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIntakeDoesNotAskTheModelWithoutEvidence(t *testing.T) {
	spec, err := ParseSpec([]byte(`{"types":{"text":{"schema":{"type":"string"}},"paper":{"fields":{"summary":{"type":"text"}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"", " \n\t ", "A fictional Quist transcript."} {
		calls := 0
		in := NewIntake(IntakeConfig{Samples: 1}, func() *Spec { return spec }, nil)
		in.http = &http.Client{Transport: evidenceTransport(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("offline")), Header: make(http.Header)}, nil
		})}
		p, _ := json.Marshal(map[string]any{"text": text})
		_, err := in.Ingest(context.Background(), &Raw{Payload: p}, "paper")
		if strings.TrimSpace(text) == "" {
			if calls != 0 || !errors.Is(err, ErrConflict) {
				t.Fatalf("empty evidence: %d model calls, %v", calls, err)
			}
		} else if calls != 1 {
			t.Fatalf("transcript was not sent to the reader: %v", err)
		}
	}
}
