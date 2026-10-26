package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func officeMedia(t *testing.T, name string, b []byte) (map[string]any, string, error) {
	t.Helper()
	dir := t.TempDir()
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	store := &dirBlobs{dir: dir}
	if err := os.MkdirAll(filepath.Dir(store.path(hash)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path(hash), b, 0o600); err != nil {
		t.Fatal(err)
	}
	a := &Archive{blobs: store, intake: &Intake{}}
	return a.media(context.Background(), &File{Name: name, MIME: mediaSniff(name, b[:min(512, len(b))]), Hash: hash, Size: int64(len(b))})
}

func officeZip(t *testing.T, entries ...[2]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, e := range entries {
		f, err := w.Create(e[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, e[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestOfficeFilesKeepSearchableWordsAndSourceLocations(t *testing.T) {
	for _, tc := range []struct{ name, words, location string }{
		{"picnic-notes.docx", "QUIST-DOCX-01", "Paragraph"},
		{"picnic-supplies.xlsx", "Blankets", `Sheet "Supplies"!A6`},
		{"picnic-plan.pptx", "Quist", "Slide 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", "office", tc.name))
			if err != nil {
				t.Fatal(err)
			}
			meta, text, err := officeMedia(t, tc.name, b)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(text, tc.words) || !strings.Contains(text, tc.location) {
				t.Errorf("missing searchable words or source location: %q", text)
			}
			fromFolder := folderText(context.Background(), tc.name, folderMIME(tc.name, b[:min(512, len(b))]), b)
			if fromFolder != text {
				t.Errorf("folder intake lost the same Office evidence: %q", fromFolder)
			}
			encoded, _ := json.Marshal(meta)
			var m struct {
				Extraction struct{ Status string }
				Passages   []struct{ Locator, Text string }
			}
			json.Unmarshal(encoded, &m)
			if m.Extraction.Status != "ready" || len(m.Passages) == 0 {
				t.Errorf("extraction metadata = %s", encoded)
			}
			for _, p := range m.Passages {
				if p.Locator == "" || p.Text == "" {
					t.Error("passage lacks a locator or words")
				}
			}
		})
	}
}

func TestOfficeArchiveLimitsNeverProducePartialEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		entries      [][2]string
	}{
		{"path traversal", "invalid", [][2]string{{"../word/document.xml", "<document/>"}}},
		{"duplicate entry", "invalid", [][2]string{{"word/document.xml", "<document/>"}, {"word/document.xml", "<document/>"}}},
		{"expansion bomb", "too_large", [][2]string{{"word/document.xml", strings.Repeat(" ", 17<<20)}}},
		{"XML entity", "invalid", [][2]string{{"word/document.xml", `<!DOCTYPE x [<!ENTITY private SYSTEM "file:///etc/passwd">]><document><p><t>&private;</t></p></document>`}}},
		{"deep XML", "too_large", [][2]string{{"word/document.xml", strings.Repeat("<p>", 100) + strings.Repeat("</p>", 100)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta, text, _ := officeMedia(t, "note.docx", officeZip(t, tc.entries...))
			out, _ := json.Marshal(meta)
			var m struct{ Extraction struct{ Status string } }
			json.Unmarshal(out, &m)
			if text != "" || m.Extraction.Status != tc.status {
				t.Errorf("got status %q and %d evidence bytes, want %s and none", m.Extraction.Status, len(text), tc.status)
			}
		})
	}
	entries := make([][2]string, 2049)
	for i := range entries {
		entries[i] = [2]string{fmt.Sprintf("many/%d", i), ""}
	}
	meta, text, _ := officeMedia(t, "many.docx", officeZip(t, entries...))
	b, _ := json.Marshal(meta)
	if text != "" || !bytes.Contains(b, []byte(`"status":"too_large"`)) {
		t.Errorf("entry count was not bounded: %s", b)
	}
}

func TestOfficeEncryptionIsIdentifiedWithoutMislabelingLegacyContainers(t *testing.T) {
	ole := []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}
	for _, tc := range []struct{ marker, status string }{{"", "unsupported"}, {"EncryptedPackage", "encrypted"}} {
		b := bytes.Clone(ole)
		for _, c := range tc.marker {
			b = append(b, byte(c), 0)
		}
		meta, text, _ := officeMedia(t, "protected.docx", b)
		out, _ := json.Marshal(meta)
		var m struct{ Extraction struct{ Status string } }
		json.Unmarshal(out, &m)
		if m.Extraction.Status != tc.status || text != "" {
			t.Errorf("container status = %s, want %s without invented evidence", out, tc.status)
		}
	}
}

func TestOfficeRelationshipsDetermineLocationsAndNeverFetchExternalParts(t *testing.T) {
	workbook := `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Quist Supplies" sheetId="7" r:id="second"/></sheets></workbook>`
	rels := `<Relationships><Relationship Id="second" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet7.xml"/></Relationships>`
	sheet := `<worksheet><sheetData><row r="4"><c r="B4" t="s"><v>0</v></c><c r="C4"><f>1+2</f><v>3</v></c></row></sheetData></worksheet>`
	entries := [][2]string{{"xl/workbook.xml", workbook}, {"xl/_rels/workbook.xml.rels", rels}, {"xl/worksheets/sheet7.xml", sheet}, {"xl/sharedStrings.xml", `<sst><si><r><t>Green </t></r><r><t>notebook</t></r></si></sst>`}}
	_, text, err := officeMedia(t, "supplies.xlsx", officeZip(t, entries...))
	if err != nil || !strings.Contains(text, `[Sheet "Quist Supplies"!B4]`+"\nGreen notebook") || !strings.Contains(text, "3 (formula: 1+2)") {
		t.Fatalf("spreadsheet evidence = %q, %v", text, err)
	}
	entries[1][1] = strings.Replace(rels, `Target="worksheets/sheet7.xml"`, `Target="https://example.invalid/private" TargetMode="External"`, 1)
	meta, text, err := officeMedia(t, "external.xlsx", officeZip(t, entries...))
	b, _ := json.Marshal(meta)
	if err == nil || text != "" || !bytes.Contains(b, []byte(`"status":"invalid"`)) {
		t.Fatalf("external part was accepted: %s, %q, %v", b, text, err)
	}

	main := `<presentation xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sldIdLst><sldId id="9" r:id="last"/><sldId id="1" r:id="first"/></sldIdLst></presentation>`
	rels = `<Relationships><Relationship Id="last" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide9.xml"/><Relationship Id="first" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`
	_, text, err = officeMedia(t, "ordered.pptx", officeZip(t, [2]string{"ppt/presentation.xml", main}, [2]string{"ppt/_rels/presentation.xml.rels", rels}, [2]string{"ppt/slides/slide9.xml", `<sld><p><r><t>First shown</t></r></p></sld>`}, [2]string{"ppt/slides/slide1.xml", `<sld><p><r><t>Second shown</t></r></p></sld>`}))
	if err != nil || !strings.HasPrefix(text, "[Slide 1 / Paragraph 1]\nFirst shown") || !strings.Contains(text, "[Slide 2 / Paragraph 1]\nSecond shown") {
		t.Fatalf("slide order = %q, %v", text, err)
	}
}
