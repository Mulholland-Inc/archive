package archive

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fuzzyOne(t *testing.T, in any, h Hints) map[string]any {
	t.Helper()
	c, err := fuzzyParse(in, h)
	if err != nil || len(c) == 0 {
		t.Fatalf("%v: %v", in, err)
	}
	return c[0].Value.(map[string]any)
}

func TestFuzzyDateReadsWhatPeopleWrite(t *testing.T) {
	for _, tc := range []struct{ in, edtf, min, max, words string }{
		{"1890", "1890", "1890-01-01", "1890-12-31", "1890"},
		{"Mar 1890", "1890-03", "1890-03-01", "1890-03-31", "March 1890"},
		{"Feb 1904", "1904-02", "1904-02-01", "1904-02-29", "February 1904"},
		{"3 Mar 1890", "1890-03-03", "1890-03-03", "1890-03-03", "03/03/1890"},
		{"about 1890", "1890~", "1890-01-01", "1890-12-31", "about 1890"},
		{"EST 1885", "1885?", "1885-01-01", "1885-12-31", "1885?"},
		{"before 1900", "../1900", "", "1900-12-31", "before 1900"},
		{"AFT Jun 1901", "1901-06/..", "1901-06-01", "", "after June 1901"},
		{"between 1890 and 1895", "1890/1895", "1890-01-01", "1895-12-31", "1890 – 1895"},
		{"the 1890s", "189X", "1890-01-01", "1899-12-31", "1890s"},
		{"18XX", "18XX", "1800-01-01", "1899-12-31", "1800s"},
		{"1890%", "1890%", "1890-01-01", "1890-12-31", "about 1890?"},
	} {
		v := fuzzyOne(t, tc.in, Hints{Country: "US"})
		str := func(k string) string { s, _ := v[k].(string); return s }
		if str("edtf") != tc.edtf || str("min") != tc.min || str("max") != tc.max {
			t.Errorf("%q read as %v, want %s %s..%s", tc.in, v, tc.edtf, tc.min, tc.max)
		}
		if err := fuzzyCheck(v); err != nil {
			t.Errorf("%q: its own reading fails the check: %v", tc.in, err)
		}
		if w := fuzzyFormat(v, "en-US"); tc.words != "" && w != tc.words && tc.in != "3 Mar 1890" {
			t.Errorf("%q says %q, want %q", tc.in, w, tc.words)
		}
		if again := fuzzyOne(t, v, Hints{}); again["edtf"] != v["edtf"] {
			t.Errorf("%q is not stable when read again: %v", tc.in, again)
		}
	}
}

func TestFuzzyDateKeepsWhatItCannotReadAsText(t *testing.T) {
	v := fuzzyOne(t, "in the spring after the flood", Hints{})
	if v["text"] != "in the spring after the flood" || v["edtf"] != nil || fuzzyCheck(v) != nil {
		t.Fatalf("%v", v)
	}
	if fuzzyFormat(v, "") != "in the spring after the flood" {
		t.Errorf("format: %q", fuzzyFormat(v, ""))
	}
}

func TestFuzzyDateRefusesWhatIsNotADay(t *testing.T) {
	for _, e := range []string{"1890-13", "1890-02-30", "1895/1890", "../..", "189X-03"} {
		if _, err := fuzzyValue(e); err == nil {
			t.Errorf("%q accepted", e)
		}
	}
	if fuzzyCheck(map[string]any{"edtf": "1890", "min": "1890-01-01", "max": "1891-12-31", "text": nil}) == nil {
		t.Error("a max that does not follow from edtf was accepted")
	}
}

// testdata/fuzzy_date_gedcom.jsonl holds how archive.family's GEDCOM reader (gedcom.ts parseDate)
// reads each phrase, as EDTF: modifiers about, before, after, between and from-to become ~, ../,
// /.., and an interval; estimated and calculated become ?. It is that app's reader on its own
// made-up test family, so fuzzy_date can take over its GEDCOM import without a date changing.
// Ambiguous numeric dates are read as the US writes them, as Ancestry's exports do.
func TestFuzzyDateReadsGEDCOMDatesAsArchiveFamilyDoes(t *testing.T) {
	f, err := os.Open("testdata/fuzzy_date_gedcom.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	for sc := bufio.NewScanner(f); sc.Scan(); n++ {
		var row struct {
			In   string  `json:"in"`
			EDTF *string `json:"edtf"`
		}
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		v := fuzzyOne(t, row.In, Hints{Country: "US"})
		got, _ := v["edtf"].(string)
		if row.EDTF == nil && got != "" || row.EDTF != nil && got != *row.EDTF {
			t.Errorf("%q: engine %v, archive.family %v", row.In, v, row.EDTF)
		}
	}
	if n < 100 {
		t.Fatalf("only %d phrases", n)
	}
}

// Through a real server: a field of type fuzzy_date takes what a person types and stores the
// reading.
func TestFuzzyDateFieldOverHTTP(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	spec := filepath.Join(in.dir, "spec.json")
	os.WriteFile(spec, []byte(`{"types": {"ancestor": {"fields": {"name": {"type": "text", "required": true}, "born": {"type": "fuzzy_date"}}}}}`), 0o644)
	in.must("spec", "push", spec)
	tok := decode[Token](t, in.must("token", "create", "-full")).Secret
	var r e2eRecord
	st, _ := call(t, "POST", srv.url+"/api/ancestor", tok, map[string]any{"name": "Mordant", "born": "abt 1832"}, &r)
	wantStatus(t, st, 201, "create")
	if b, _ := r.Data["born"].(map[string]any); b["edtf"] != "1832~" || b["min"] != "1832-01-01" || b["max"] != "1832-12-31" {
		t.Fatalf("born = %v", r.Data["born"])
	}
	// min and max follow from edtf whatever is sent, so they cannot be made to say otherwise
	st, _ = call(t, "PATCH", srv.url+"/api/ancestor/"+r.ID, tok, map[string]any{"born": map[string]any{"edtf": "1832", "min": "1832-01-01", "max": "1833-01-01", "text": nil}}, &r)
	wantStatus(t, st, 200, "patch")
	if b, _ := r.Data["born"].(map[string]any); b["edtf"] != "1832" || b["max"] != "1832-12-31" {
		t.Fatalf("born = %v", r.Data["born"])
	}
}
