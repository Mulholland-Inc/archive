package archive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOnlyTheOwnerCanRetryExtractionWithoutChangingTheOriginal(t *testing.T) {
	// Keep this capability assertion so the test reproduces the missing API before its implementation.
	type retrying interface {
		RetryFile(context.Context, uuid.UUID) (*File, error)
	}
	if _, ok := any(&Archive{}).(retrying); !ok {
		t.Fatal("the engine cannot retry extraction of a saved file")
	}
	in := newInst(t)
	t.Setenv("ARCHIVE_DATA", in.dir)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ARCHIVE_SECRET_KEY", "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	a, err := Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err = a.AddUser(ctx, "wren@example.com", "Wren Quist", true); err != nil {
		t.Fatal(err)
	}
	actor, err := a.CLIActor(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx = WithActor(ctx, actor)
	if _, err = a.AddUser(ctx, "rook@example.com", "Rook Quist", false); err != nil {
		t.Fatal(err)
	}
	if _, err = a.AddSpace(ctx, "papers"); err != nil {
		t.Fatal(err)
	}
	if err = a.SetMember(ctx, "papers", "rook@example.com", "editor"); err != nil {
		t.Fatal(err)
	}
	body := []byte("A fictional Quist paper preserved through retries.")
	f, err := a.AddFile(ctx, "papers", "paper.txt", "", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.ProcessFile(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	retry := any(a).(retrying)
	rook, err := a.CLIActor(ctx, "rook@example.com")
	if err != nil {
		t.Fatal(err)
	}
	other := WithActor(ctx, rook)
	if _, err = a.GetFile(other, f.ID); err != nil {
		t.Fatalf("fixture must let the other member see the file: %v", err)
	}
	if _, err = retry.RetryFile(other, f.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("another editor retried extraction: %v", err)
	}
	for _, status := range []string{"error", "done", "processing"} {
		if err = a.sys(ctx, func(ctx context.Context) error {
			_, e := a.st.Q(ctx).Exec(ctx, `UPDATE files SET status=$2, error='fixture failure' WHERE id=$1`, f.ID, status)
			return e
		}); err != nil {
			t.Fatal(err)
		}
		got, err := retry.RetryFile(ctx, f.ID)
		if status == "processing" {
			if !errors.Is(err, ErrConflict) {
				t.Fatalf("active extraction was restarted: %v", err)
			}
		} else {
			if err != nil || got.Status != "pending" || got.Error != "" {
				t.Fatalf("retry from %s: %+v %v", status, got, err)
			}
			if _, err = a.ProcessFile(ctx, f.ID); err != nil {
				t.Fatal(err)
			}
			text, err := a.FileText(ctx, f.ID)
			if err != nil || text != string(body) {
				t.Fatalf("retried text = %q: %v", text, err)
			}
		}
		got, rc, err := a.OpenFile(ctx, f.ID)
		if err != nil {
			t.Fatal(err)
		}
		original, err := io.ReadAll(rc)
		rc.Close()
		if err != nil || !bytes.Equal(original, body) || got.Hash != f.Hash || got.Owner != f.Owner || got.Space != f.Space {
			t.Fatal("retry changed the original or its owner/space")
		}
	}
}

type fileRetryTransport func(*http.Request) (*http.Response, error)

func (f fileRetryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEncryptedFileEvidenceNeverReachesTheReader(t *testing.T) {
	in := newInst(t)
	t.Setenv("ARCHIVE_DATA", in.dir)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ARCHIVE_SECRET_KEY", "")
	a, err := Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err = a.AddUser(context.Background(), "wren@example.com", "Wren Quist", true); err != nil {
		t.Fatal(err)
	}
	actor, err := a.CLIActor(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithActor(context.Background(), actor)
	spec, err := ParseSpec([]byte(`{"types":{"paper":{"fields":{"title":{"type":"text"}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	for name, typ := range a.Spec().Types {
		if spec.Types[name] == nil {
			spec.Types[name] = typ
		}
	}
	b, _ := spec.JSON()
	if _, err = a.ApplySpec(ctx, b, false); err != nil {
		t.Fatal(err)
	}
	if _, err = a.AddSpace(ctx, "papers"); err != nil {
		t.Fatal(err)
	}
	f, err := a.AddFile(ctx, "papers", "Quist-locked.pdf", "", bytes.NewBufferString("%PDF-1.4\nFictional encrypted document"))
	if err != nil {
		t.Fatal(err)
	}
	// Represent the persisted result of a PDF extractor that confirmed encryption.
	err = a.sys(ctx, func(ctx context.Context) error {
		_, e := a.st.Q(ctx).Exec(ctx, `UPDATE files SET mime='application/pdf',status='error',text='',metadata='{"extraction":{"status":"encrypted"}}'::jsonb WHERE id=$1`, f.ID)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	a.intake.http = &http.Client{Transport: fileRetryTransport(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("no model is called by this fixture")
	})}
	_, err = a.IntakeFile(ctx, f.ID, "paper")
	if !errors.Is(err, ErrConflict) || requests != 0 {
		t.Fatalf("encrypted evidence reached the reader: requests=%d err=%v", requests, err)
	}
}
