package archive

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Office containers are untrusted uploads. Nothing is extracted to disk, no relationship
// is fetched, and package expansion, XML work, and the resulting evidence are bounded.
const (
	officeEntries     = 2048
	officePartMax     = 16 << 20
	officeTotalMax    = 64 << 20
	officeTextMax     = 2 << 20
	officePassagesMax = 20000
)

type FilePassage struct {
	Locator string `json:"locator"`
	Text    string `json:"text"`
}

type extractionError struct{ status, reason string }

func (e *extractionError) Error() string        { return e.reason }
func officeFailure(status, reason string) error { return &extractionError{status, reason} }
func extractionStatus(status, format, reason string) map[string]any {
	v := map[string]any{"status": status, "format": format}
	if reason != "" {
		v["reason"] = reason
	}
	return v
}
func officeFormat(name string) string {
	switch ext := strings.ToLower(filepath.Ext(name)); ext {
	case ".docx", ".xlsx", ".pptx":
		return ext[1:]
	}
	return ""
}
func officeMIME(format string) string {
	return map[string]string{"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation"}[format]
}

type officePackage struct{ files map[string]*zip.File }

func openOffice(b []byte) (*officePackage, error) {
	if bytes.HasPrefix(b, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
		// Modern encrypted Office packages use named streams in an OLE container. A
		// legacy binary document renamed .docx is unsupported, not proof of encryption.
		for _, name := range []string{"EncryptedPackage", "EncryptionInfo"} {
			var utf16 []byte
			for _, c := range name {
				utf16 = append(utf16, byte(c), 0)
			}
			if bytes.Contains(b, utf16) {
				return nil, officeFailure("encrypted", "This Office package is encrypted; save an unlocked copy to read it.")
			}
		}
		return nil, officeFailure("unsupported", "This is a legacy binary Office container; save a modern Office copy to read it.")
	}
	if err := CheckZIPDirectory(bytes.NewReader(b), int64(len(b)), officeEntries, 8<<20); err != nil {
		if errors.Is(err, ErrZIPLimit) {
			return nil, officeFailure("too_large", "The Office package directory exceeds the extraction limit.")
		}
		return nil, officeFailure("invalid", "The Office package could not be opened.")
	}
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, officeFailure("invalid", "The Office package could not be opened.")
	}
	if len(z.File) > officeEntries {
		return nil, officeFailure("too_large", "The Office package has too many entries.")
	}
	out := &officePackage{files: map[string]*zip.File{}}
	var total uint64
	for _, f := range z.File {
		n := strings.TrimSuffix(f.Name, "/")
		if n == "" || path.Clean(n) != n || strings.HasPrefix(n, "/") || strings.ContainsAny(n, "\\:\x00") || n == ".." || strings.HasPrefix(n, "../") {
			return nil, officeFailure("invalid", "The Office package contains an unsafe path.")
		}
		if _, exists := out.files[f.Name]; exists {
			return nil, officeFailure("invalid", "The Office package contains duplicate entries.")
		}
		if f.Flags&1 != 0 {
			return nil, officeFailure("encrypted", "The Office package is encrypted.")
		}
		if f.Method != zip.Store && f.Method != zip.Deflate {
			return nil, officeFailure("unsupported", "The Office package uses unsupported compression.")
		}
		if f.UncompressedSize64 > officePartMax || total > officeTotalMax-f.UncompressedSize64 {
			return nil, officeFailure("too_large", "The expanded Office package exceeds the extraction limit.")
		}
		total += f.UncompressedSize64
		out.files[f.Name] = f
	}
	return out, nil
}
func (p *officePackage) read(name string) ([]byte, error) {
	f := p.files[name]
	if f == nil {
		return nil, officeFailure("invalid", "The Office package is missing a required part.")
	}
	r, err := f.Open()
	if err != nil {
		return nil, officeFailure("invalid", "An Office part could not be opened.")
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, officePartMax+1))
	if len(b) > officePartMax {
		return nil, officeFailure("too_large", "An Office part exceeds the extraction limit.")
	}
	if err != nil {
		return nil, officeFailure("invalid", "An Office part failed its integrity check.")
	}
	return b, nil
}

// xml.Decoder does not fetch entities. Reject directives as well, and cap nesting and tokens
// so adversarial markup cannot turn a small archive into unbounded parsing or output work.
func officeXML(b []byte, visit func(xml.Token) error) error {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	d := xml.NewDecoder(bytes.NewReader(b))
	depth, roots, tokens := 0, 0, 0
	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return officeFailure("invalid", "An Office XML part could not be read.")
		}
		tokens++
		if tokens > 1000000 {
			return officeFailure("too_large", "An Office XML part is too complex.")
		}
		switch x := t.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
			if depth > 64 || len(x.Attr) > 128 {
				return officeFailure("too_large", "An Office XML part is too deeply nested.")
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			return officeFailure("invalid", "Office XML directives are not supported.")
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(x)) != "" {
				return officeFailure("invalid", "An Office XML part has text outside its root.")
			}
		}
		if err := visit(t); err != nil {
			return err
		}
	}
	if roots != 1 || depth != 0 {
		return officeFailure("invalid", "An Office XML part has no single root.")
	}
	return nil
}
func officeAttr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

type officeEvidence struct {
	passages []FilePassage
	bytes    int
}

func (e *officeEvidence) add(locator, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(e.passages) >= officePassagesMax || e.bytes+len(locator)+len(text)+4 > officeTextMax {
		return officeFailure("too_large", "The Office text exceeds the extraction limit.")
	}
	e.bytes += len(locator) + len(text) + 4
	e.passages = append(e.passages, FilePassage{locator, text})
	return nil
}
func (e *officeEvidence) text() string {
	var b strings.Builder
	for _, p := range e.passages {
		fmt.Fprintf(&b, "[%s]\n%s\n\n", p.Locator, p.Text)
	}
	return strings.TrimSpace(b.String())
}
func (p *officePackage) paragraphs(name, label string, evidence *officeEvidence) error {
	b, err := p.read(name)
	if err != nil {
		return err
	}
	type paragraph struct {
		text strings.Builder
		n    int
	}
	var stack []*paragraph
	n, inText, deleted := 0, 0, 0
	return officeXML(b, func(t xml.Token) error {
		switch x := t.(type) {
		case xml.StartElement:
			switch x.Name.Local {
			case "del":
				deleted++
			case "p":
				n++
				stack = append(stack, &paragraph{n: n})
			case "t":
				inText++
			case "tab", "br", "cr":
				if len(stack) > 0 && deleted == 0 {
					stack[len(stack)-1].text.WriteByte('\n')
				}
			}
		case xml.CharData:
			if len(stack) > 0 && inText > 0 && deleted == 0 {
				stack[len(stack)-1].text.Write(x)
			}
		case xml.EndElement:
			switch x.Name.Local {
			case "del":
				deleted--
			case "t":
				inText--
			case "p":
				if len(stack) > 0 {
					v := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					return evidence.add(fmt.Sprintf("%sParagraph %d", label, v.n), v.text.String())
				}
			}
		}
		return nil
	})
}

type officeRelationship struct {
	target, kind string
	external     bool
}

func (p *officePackage) relationships(main string) (map[string]officeRelationship, error) {
	name := path.Join(path.Dir(main), "_rels", path.Base(main)+".rels")
	b, err := p.read(name)
	if err != nil {
		return nil, err
	}
	out := map[string]officeRelationship{}
	err = officeXML(b, func(t xml.Token) error {
		x, ok := t.(xml.StartElement)
		if !ok || x.Name.Local != "Relationship" {
			return nil
		}
		id, target := officeAttr(x, "Id"), officeAttr(x, "Target")
		if _, exists := out[id]; id == "" || exists {
			return officeFailure("invalid", "An Office relationship is missing or duplicated.")
		}
		out[id] = officeRelationship{target, officeAttr(x, "Type"), strings.EqualFold(officeAttr(x, "TargetMode"), "External")}
		return nil
	})
	return out, err
}
func officeTarget(main string, rel officeRelationship, kind string) (string, error) {
	if rel.external || rel.target == "" || !strings.HasSuffix(rel.kind, "/"+kind) {
		return "", officeFailure("invalid", "An Office content relationship is unavailable or external.")
	}
	if strings.ContainsAny(rel.target, "\\:\x00?#") {
		return "", officeFailure("invalid", "An Office relationship has an unsafe target.")
	}
	n := path.Clean(path.Join(path.Dir(main), rel.target))
	if strings.HasPrefix(rel.target, "/") {
		n = path.Clean(strings.TrimPrefix(rel.target, "/"))
	}
	if !strings.HasPrefix(n, strings.Split(main, "/")[0]+"/") {
		return "", officeFailure("invalid", "An Office relationship leaves its package area.")
	}
	return n, nil
}
func (p *officePackage) sharedStrings() ([]string, error) {
	if p.files["xl/sharedStrings.xml"] == nil {
		return nil, nil
	}
	b, err := p.read("xl/sharedStrings.xml")
	if err != nil {
		return nil, err
	}
	var out []string
	var s strings.Builder
	inText, total := 0, 0
	err = officeXML(b, func(t xml.Token) error {
		switch x := t.(type) {
		case xml.StartElement:
			if x.Name.Local == "si" {
				s.Reset()
			}
			if x.Name.Local == "t" {
				inText++
			}
		case xml.CharData:
			if inText > 0 {
				s.Write(x)
			}
		case xml.EndElement:
			if x.Name.Local == "t" {
				inText--
			}
			if x.Name.Local == "si" {
				total += s.Len()
				if total > officeTextMax || len(out) >= officePassagesMax {
					return officeFailure("too_large", "The spreadsheet string table exceeds the extraction limit.")
				}
				out = append(out, s.String())
			}
		}
		return nil
	})
	return out, err
}
func (p *officePackage) sheet(name, label string, shared []string, evidence *officeEvidence) error {
	b, err := p.read(name)
	if err != nil {
		return err
	}
	var value, formula strings.Builder
	cell, kind, field := "", "", ""
	row, column := 0, 0
	return officeXML(b, func(t xml.Token) error {
		switch x := t.(type) {
		case xml.StartElement:
			switch x.Name.Local {
			case "row":
				row++
				column = 0
				if n, e := strconv.Atoi(officeAttr(x, "r")); e == nil && n > 0 {
					row = n
				}
			case "c":
				column++
				cell, kind = officeAttr(x, "r"), officeAttr(x, "t")
				if cell == "" {
					cell = officeColumn(column) + strconv.Itoa(row)
				}
				value.Reset()
				formula.Reset()
			case "v", "t", "f":
				field = x.Name.Local
			}
		case xml.CharData:
			if cell != "" {
				if field == "f" {
					formula.Write(x)
				} else if field == "v" || field == "t" {
					value.Write(x)
				}
			}
		case xml.EndElement:
			switch x.Name.Local {
			case "v", "t", "f":
				field = ""
			case "c":
				v := value.String()
				if kind == "s" {
					n, e := strconv.Atoi(v)
					if e != nil || n < 0 || n >= len(shared) {
						return officeFailure("invalid", "A spreadsheet cell refers to a missing shared string.")
					}
					v = shared[n]
				}
				if kind == "b" {
					if v == "1" {
						v = "TRUE"
					} else if v == "0" {
						v = "FALSE"
					}
				}
				if formula.Len() > 0 {
					v = strings.TrimSpace(v + " (formula: " + formula.String() + ")")
				}
				if err := evidence.add(fmt.Sprintf("Sheet %q!%s", label, cell), v); err != nil {
					return err
				}
				cell = ""
			}
		}
		return nil
	})
}
func officeColumn(n int) string {
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}

func mediaOffice(name string, b []byte) (meta map[string]any, text string, err error) {
	format := officeFormat(name)
	meta = map[string]any{}
	defer func() {
		status, reason := "ready", ""
		if err != nil {
			status, reason = "invalid", err.Error()
			var failure *extractionError
			if errors.As(err, &failure) {
				status = failure.status
			}
			text = ""
			delete(meta, "passages")
		} else if text == "" {
			status = "empty"
			reason = "No readable text was found in this Office file."
		}
		meta["extraction"] = extractionStatus(status, format, reason)
	}()
	p, err := openOffice(b)
	if err != nil {
		return meta, "", err
	}
	evidence := &officeEvidence{}
	switch format {
	case "docx":
		err = p.paragraphs("word/document.xml", "", evidence)
		if err == nil {
			var extras []string
			for n := range p.files {
				if strings.HasSuffix(n, ".xml") && (strings.HasPrefix(n, "word/header") || strings.HasPrefix(n, "word/footer") || n == "word/footnotes.xml" || n == "word/endnotes.xml") {
					extras = append(extras, n)
				}
			}
			sort.Strings(extras)
			for _, n := range extras {
				if err = p.paragraphs(n, path.Base(n)+" / ", evidence); err != nil {
					break
				}
			}
		}
	case "xlsx", "pptx":
		main, entry, kind := "xl/workbook.xml", "sheet", "worksheet"
		if format == "pptx" {
			main, entry, kind = "ppt/presentation.xml", "sldId", "slide"
		}
		var rels map[string]officeRelationship
		rels, err = p.relationships(main)
		if err != nil {
			break
		}
		var shared []string
		if format == "xlsx" {
			shared, err = p.sharedStrings()
			if err != nil {
				break
			}
		}
		var data []byte
		data, err = p.read(main)
		if err != nil {
			break
		}
		n := 0
		err = officeXML(data, func(t xml.Token) error {
			x, ok := t.(xml.StartElement)
			if !ok || x.Name.Local != entry {
				return nil
			}
			n++
			id := officeAttr(x, "id")
			// Presentation slide IDs carry both a numeric id and a relationship id.
			for _, a := range x.Attr {
				if a.Name.Local == "id" && strings.HasSuffix(a.Name.Space, "/relationships") {
					id = a.Value
				}
			}
			target, e := officeTarget(main, rels[id], kind)
			if e != nil {
				return e
			}
			if format == "xlsx" {
				return p.sheet(target, officeAttr(x, "name"), shared, evidence)
			}
			return p.paragraphs(target, fmt.Sprintf("Slide %d / ", n), evidence)
		})
	}
	if err != nil {
		return meta, "", err
	}
	meta["passages"] = evidence.passages
	return meta, evidence.text(), nil
}
