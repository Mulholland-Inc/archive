package main

import (
	"strings"
	"testing"
)

func TestBrowseNeedsTheBrowserAndComesBeforeWrites(t *testing.T) {
	t.Parallel()
	run := func(in *e2eInst, code string) RunOutput {
		t.Helper()
		out, errs, err := in.run(code, "run", "-")
		if err != nil && out == "" {
			t.Fatalf("archive run: %v\n%s", err, errs)
		}
		return decode[RunOutput](t, out)
	}
	off := newInst(t)
	off.must("user", "add", "alice@example.com")
	if out := run(off, `return await archive.browse("Find the Learn more link", { url: "https://example.com" })`); !strings.Contains(out.Error, "browse is not configured") {
		t.Errorf("unconfigured: %+v", out)
	}
	on := newInst(t, "ARCHIVE_BROWSER=http://127.0.0.1:1", "TYPESAFE_API_KEY=x", "ANTHROPIC_API_KEY=x")
	on.must("user", "add", "alice@example.com")
	on.push(map[string]string{"house.sql": e2eHouse})
	if out := run(on, `await archive.create("house", { address: "Canal 1" });
return await archive.browse("Find the Learn more link")`); !strings.Contains(out.Error, "browse before you write") {
		t.Errorf("after a write: %+v", out)
	}
	if out := run(on, `await archive.migrate("ALTER TABLE house ADD COLUMN rooms integer");
return await archive.browse("Find the Learn more link")`); !strings.Contains(out.Error, "browse before you write") {
		t.Errorf("after a migration: %+v", out)
	}
	if out := run(on, `await archive.list("house");
await archive.sql("SELECT count(*) FROM house");
return await archive.browse("Find the Learn more link")`); !strings.Contains(out.Error, "could not start the browser") {
		t.Errorf("after reads: %+v", out)
	}
}
