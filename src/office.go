package main

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type OfficeNode struct {
	Prefix, Local  string
	Start, End     int
	InStart, InEnd int
	Attr           []xml.Attr
	Text           string
	Parent         *OfficeNode
	Kids           []*OfficeNode
}

func (n *OfficeNode) Is(prefix, local string) bool {
	return n.Prefix == prefix && n.Local == local
}
func (n *OfficeNode) Get(name string) (string, bool) {
	p, l, ok := strings.Cut(name, ":")
	if !ok {
		p, l = "", name
	}
	for _, a := range n.Attr {
		if a.Name.Space == p && a.Name.Local == l {
			return a.Value, true
		}
	}
	return "", false
}
func (n *OfficeNode) Attrv(name string) string {
	v, _ := n.Get(name)
	return v
}
func (n *OfficeNode) SelfClosing() bool {
	return n.InStart == n.End
}
func (n *OfficeNode) Child(prefix, local string) *OfficeNode {
	for _, k := range n.Kids {
		if k.Is(prefix, local) {
			return k
		}
	}
	return nil
}
func (n *OfficeNode) Children(prefix, local string) []*OfficeNode {
	var out []*OfficeNode
	for _, k := range n.Kids {
		if k.Is(prefix, local) {
			out = append(out, k)
		}
	}
	return out
}
func (n *OfficeNode) Find(prefix, local string) []*OfficeNode {
	var out []*OfficeNode
	var walk func(*OfficeNode)
	walk = func(x *OfficeNode) {
		for _, k := range x.Kids {
			if k.Is(prefix, local) {
				out = append(out, k)
			}
			walk(k)
		}
	}
	walk(n)
	return out
}
func (n *OfficeNode) Nearest(prefix, local string) *OfficeNode {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Is(prefix, local) {
			return p
		}
	}
	return nil
}
func OfficeParse(b []byte) (*OfficeNode, error) {
	d := xml.NewDecoder(bytes.NewReader(b))
	doc := &OfficeNode{End: len(b), InEnd: len(b)}
	cur := doc
	for {
		off := int(d.InputOffset())
		tok, err := d.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &OfficeNode{Prefix: t.Name.Space, Local: t.Name.Local, Start: off, InStart: int(d.InputOffset()), Attr: append([]xml.Attr(nil), t.Attr...), Parent: cur}
			cur.Kids = append(cur.Kids, n)
			cur = n
		case xml.EndElement:
			if cur == doc || cur.Prefix != t.Name.Space || cur.Local != t.Name.Local {
				return nil, fmt.Errorf("mismatched end tag %s:%s at %d", t.Name.Space, t.Name.Local, off)
			}
			cur.InEnd, cur.End = off, int(d.InputOffset())
			cur = cur.Parent
		case xml.CharData:
			cur.Text += string(t)
		}
	}
	if cur != doc || len(doc.Kids) == 0 {
		return nil, fmt.Errorf("unclosed or empty document")
	}
	return doc, nil
}
func OfficeRoot(doc *OfficeNode) *OfficeNode {
	return doc.Kids[0]
}
func OfficePrefixOf(doc *OfficeNode, ns ...string) (string, bool) {
	for _, a := range OfficeRoot(doc).Attr {
		for _, s := range ns {
			if a.Value == s {
				if a.Name.Space == "xmlns" {
					return a.Name.Local, true
				}
				if a.Name.Space == "" && a.Name.Local == "xmlns" {
					return "", true
				}
			}
		}
	}
	return "", false
}
func OfficeWellFormed(b []byte) error {
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		_, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

type OfficeSplice struct {
	Start, End int
	New        []byte
}

func OfficeApply(b []byte, ss []OfficeSplice) ([]byte, error) {
	ss = append([]OfficeSplice(nil), ss...)
	sort.SliceStable(ss, func(i, j int) bool {
		return ss[i].Start < ss[j].Start
	})
	var out bytes.Buffer
	pos := 0
	for _, s := range ss {
		if s.Start < pos || s.End < s.Start || s.End > len(b) {
			return nil, fmt.Errorf("overlapping or invalid splice %d-%d", s.Start, s.End)
		}
		out.Write(b[pos:s.Start])
		out.Write(s.New)
		pos = s.End
	}
	out.Write(b[pos:])
	return out.Bytes(), nil
}
func OfficeEsc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
func OfficeEscAttr(s string) string {
	return strings.ReplaceAll(OfficeEsc(s), `"`, "&quot;")
}
func OfficeQ(prefix, local string) string {
	if prefix == "" {
		return local
	}
	return prefix + ":" + local
}

type OfficePackage struct {
	zr      *zip.Reader
	parts   map[string][]byte
	removed map[string]bool
	Log     map[string][][]OfficeSplice
}

func OfficeOpen(b []byte) (*OfficePackage, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	return &OfficePackage{zr: zr, parts: map[string][]byte{}, removed: map[string]bool{}, Log: map[string][][]OfficeSplice{}}, nil
}
func (p *OfficePackage) file(name string) *zip.File {
	for _, f := range p.zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}
func (p *OfficePackage) Has(name string) bool {
	return !p.removed[name] && (p.parts[name] != nil || p.file(name) != nil)
}
func (p *OfficePackage) Part(name string) ([]byte, error) {
	if p.removed[name] {
		return nil, fmt.Errorf("part %s removed", name)
	}
	if b, ok := p.parts[name]; ok {
		return b, nil
	}
	f := p.file(name)
	if f == nil {
		return nil, fmt.Errorf("part %s not found", name)
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
func (p *OfficePackage) Splice(name string, ss []OfficeSplice) error {
	if len(ss) == 0 {
		return nil
	}
	b, err := p.Part(name)
	if err != nil {
		return err
	}
	nb, err := OfficeApply(b, ss)
	if err != nil {
		return err
	}
	if err := OfficeWellFormed(nb); err != nil {
		return fmt.Errorf("edit would make %s malformed: %v", name, err)
	}
	p.parts[name] = nb
	p.Log[name] = append(p.Log[name], ss)
	return nil
}
func (p *OfficePackage) Remove(name string) {
	p.removed[name] = true
}
func (p *OfficePackage) Save(w io.Writer) error {
	zw := zip.NewWriter(w)
	for _, f := range p.zr.File {
		if p.removed[f.Name] {
			continue
		}
		b, ok := p.parts[f.Name]
		if !ok {
			if err := zw.Copy(f); err != nil {
				return err
			}
			continue
		}
		h := &zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: f.Modified}
		if f.Method == zip.Store {
			h.Method = zip.Store
		}
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := fw.Write(b); err != nil {
			return err
		}
	}
	return zw.Close()
}

type OfficeRel struct{ ID, Type, Target string }

func (p *OfficePackage) Rels(part string) ([]OfficeRel, error) {
	dir, file := path.Split(part)
	rp := dir + "_rels/" + file + ".rels"
	b, err := p.Part(rp)
	if err != nil {
		return nil, err
	}
	var x struct {
		R []struct {
			ID     string `xml:"Id,attr"`
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(b, &x); err != nil {
		return nil, err
	}
	var out []OfficeRel
	for _, r := range x.R {
		t := r.Target
		if r.Mode != "External" {
			if strings.HasPrefix(t, "/") {
				t = t[1:]
			} else {
				t = path.Join(dir, t)
			}
		}
		out = append(out, OfficeRel{r.ID, r.Type, t})
	}
	return out, nil
}
func officeRelsPath(part string) string {
	dir, file := path.Split(part)
	return dir + "_rels/" + file + ".rels"
}
func (p *OfficePackage) DropPart(name, owner string) error {
	for _, item := range []struct{ part, attr, val string }{{officeRelsPath(owner), "Target", path.Base(name)}, {"[Content_Types].xml", "PartName", "/" + name}} {
		b, err := p.Part(item.part)
		if err != nil {
			return err
		}
		doc, err := OfficeParse(b)
		if err != nil {
			return err
		}
		var ss []OfficeSplice
		for _, k := range OfficeRoot(doc).Kids {
			v := k.Attrv(item.attr)
			if v == item.val || strings.HasSuffix(v, "/"+item.val) {
				ss = append(ss, OfficeSplice{k.Start, k.End, nil})
			}
		}
		if err := p.Splice(item.part, ss); err != nil {
			return err
		}
	}
	p.Remove(name)
	return nil
}

type OfficeModel struct {
	Type       string            `json:"type"`
	Paragraphs []OfficeParagraph `json:"paragraphs,omitzero"`
	Tables     []OfficeTable     `json:"tables,omitempty"`
	Sheets     []OfficeSheet     `json:"sheets,omitempty"`
	Slides     []OfficeSlide     `json:"slides,omitempty"`
	// What OfficeView adds. Width and Height are a docx's page in twips, or a deck's slide in EMU.
	Width  int64         `json:"width,omitempty"`
	Height int64         `json:"height,omitempty"`
	Header string        `json:"header,omitempty"`
	Footer string        `json:"footer,omitempty"`
	Styles []OfficeStyle `json:"styles,omitempty"`
}
type OfficeParagraph struct {
	ID    string   `json:"id,omitempty"`
	Style string   `json:"style,omitempty"`
	List  string   `json:"list,omitempty"`
	In    string   `json:"in,omitempty"`
	Text  string   `json:"text"`
	Runs  []string `json:"runs,omitempty"`
	// What OfficeView adds. Spans are Text by formatting, when any has some; Marker is a list item's number or bullet.
	Spans  []OfficeSpan  `json:"spans,omitempty"`
	Marker string        `json:"marker,omitempty"`
	Level  int           `json:"level,omitempty"`
	Align  string        `json:"align,omitempty"`
	Images []OfficeImage `json:"images,omitempty"`
}
type OfficeSpan struct {
	Text   string  `json:"text"`
	Bold   bool    `json:"bold,omitempty"`
	Italic bool    `json:"italic,omitempty"`
	Under  bool    `json:"under,omitempty"`
	Strike bool    `json:"strike,omitempty"`
	Link   bool    `json:"link,omitempty"`
	Color  string  `json:"color,omitempty"`
	Size   float64 `json:"size,omitempty"` // points, on a slide
}
type OfficeBox struct {
	X   int64   `json:"x"`
	Y   int64   `json:"y"`
	W   int64   `json:"w"`
	H   int64   `json:"h"`
	Rot float64 `json:"rot,omitempty"`
}
type OfficeImage struct {
	Part string    `json:"part"`
	Box  OfficeBox `json:"box"`
	Z    int       `json:"z,omitempty"`
}
type OfficeGridCell struct {
	Span int    `json:"span,omitempty"`
	Rows int    `json:"rows,omitempty"`
	Skip bool   `json:"skip,omitempty"` // covered by the merged cell above
	Fill string `json:"fill,omitempty"`
}
type OfficeTable struct {
	ID   string             `json:"id"`
	Rows [][]string         `json:"rows"`
	Grid [][]OfficeGridCell `json:"grid,omitempty"`
}
type OfficeCol struct {
	Min    int     `json:"min"`
	Max    int     `json:"max"`
	Width  float64 `json:"width,omitempty"` // in characters of the default font
	Hidden bool    `json:"hidden,omitempty"`
}
type OfficeStyle struct {
	Bold   bool    `json:"bold,omitempty"`
	Italic bool    `json:"italic,omitempty"`
	Under  bool    `json:"under,omitempty"`
	Strike bool    `json:"strike,omitempty"`
	Size   float64 `json:"size,omitempty"`
	Color  string  `json:"color,omitempty"`
	Fill   string  `json:"fill,omitempty"`
	Align  string  `json:"align,omitempty"`
	VAlign string  `json:"valign,omitempty"`
	Wrap   bool    `json:"wrap,omitempty"`
	Indent int     `json:"indent,omitempty"`
	Top    string  `json:"top,omitempty"` // a border's color, "auto" when it has none of its own
	Right  string  `json:"right,omitempty"`
	Bottom string  `json:"bottom,omitempty"`
	Left   string  `json:"left,omitempty"`
}
type OfficeSheet struct {
	Name      string         `json:"name"`
	Dimension string         `json:"dimension,omitempty"`
	Merged    []string       `json:"merged,omitempty"`
	Tables    []OfficeXTable `json:"tables,omitempty"`
	Cells     []OfficeCell   `json:"cells"`
	Cols      []OfficeCol    `json:"cols,omitempty"` // OfficeView
}
type OfficeXTable struct {
	Name    string   `json:"name"`
	Ref     string   `json:"ref"`
	Columns []string `json:"columns"`
}
type OfficeCell struct {
	Ref     string `json:"ref"`
	Value   string `json:"value,omitempty"`
	Formula string `json:"formula,omitempty"`
	// What OfficeView adds: the value as its number format shows it, that format's color, and an index into Styles.
	Text  string `json:"text,omitempty"`
	Ink   string `json:"ink,omitempty"`
	Style int    `json:"style,omitempty"`
}
type OfficeSlide struct {
	Number int           `json:"slide"`
	Hidden bool          `json:"hidden,omitempty"`
	Shapes []OfficeShape `json:"shapes"`
	// What OfficeView adds: what is behind and around the shapes that hold text. Everything drawn has a Z.
	Background string        `json:"background,omitempty"`
	Backdrop   string        `json:"backdrop,omitempty"`
	Decor      []OfficeShape `json:"decor,omitempty"`
	Pictures   []OfficeImage `json:"pictures,omitempty"`
}
type OfficeShape struct {
	Index       int               `json:"shape"`
	Name        string            `json:"name,omitempty"`
	Placeholder string            `json:"placeholder,omitempty"`
	Paragraphs  []OfficeParagraph `json:"paragraphs"`
	// What OfficeView adds. Inset is the text's distance from the box's sides, in EMU: left, top, right, bottom.
	Box    *OfficeBox `json:"box,omitempty"`
	Z      int        `json:"z,omitempty"`
	Fill   string     `json:"fill,omitempty"`
	Line   string     `json:"line,omitempty"`
	Geom   string     `json:"geom,omitempty"`
	Anchor string     `json:"anchor,omitempty"`
	Inset  []int64    `json:"inset,omitempty"`
}
type OfficeOp struct {
	Op        string `json:"op"`
	Para      string `json:"para,omitempty"`
	Find      string `json:"find,omitempty"`
	Replace   string `json:"replace,omitempty"`
	All       bool   `json:"all,omitempty"`
	Text      string `json:"text,omitempty"`
	Style     string `json:"style,omitempty"`
	Table     *int   `json:"table,omitempty"`
	Row       *int   `json:"row,omitempty"`
	Col       *int   `json:"col,omitempty"`
	Sheet     string `json:"sheet,omitempty"`
	Cell      string `json:"cell,omitempty"`
	Value     any    `json:"value,omitempty"`
	TableName string `json:"table_name,omitempty"`
	Values    []any  `json:"values,omitempty"`
	Formula   string `json:"formula,omitempty"`
	Slide     int    `json:"slide,omitempty"`
	Shape     *int   `json:"shape,omitempty"`
}

var officeOpTypes = map[string]string{"replace_text": "docx pptx", "insert_paragraph_after": "docx", "delete_paragraph": "docx", "set_cell_text": "docx", "set_cell": "xlsx", "set_formula": "xlsx", "clear_cell": "xlsx", "append_table_row": "xlsx", "replace_shape_text": "pptx"}

func OfficeKind(name string) (string, error) {
	switch k := strings.ToLower(strings.TrimPrefix(path.Ext(name), ".")); k {
	case "docx", "docm", "dotx":
		return "docx", nil
	case "xlsx", "xlsm", "xltx":
		return "xlsx", nil
	case "pptx", "pptm", "potx":
		return "pptx", nil
	default:
		return "", fmt.Errorf("unsupported file type %q; expected docx, xlsx or pptx", k)
	}
}
func OfficeParseOps(b []byte) ([]OfficeOp, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var ops []OfficeOp
	if err := d.Decode(&ops); err != nil {
		return nil, fmt.Errorf("edits must be a JSON array of objects with an \"op\" field: %v", err)
	}
	return ops, nil
}
func OfficeRead(b []byte, name string) (*OfficeModel, error) {
	kind, err := OfficeKind(name)
	if err != nil {
		return nil, err
	}
	p, err := OfficeOpen(b)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "docx":
		w, err := officeOpenWord(p)
		if err != nil {
			return nil, err
		}
		return w.model(), nil
	case "xlsx":
		bk, err := officeOpenBook(p)
		if err != nil {
			return nil, err
		}
		m := &OfficeModel{Type: "xlsx"}
		for _, n := range bk.names {
			s, err := bk.sheet(n)
			if err != nil {
				continue
			}
			m.Sheets = append(m.Sheets, s.model())
		}
		return m, nil
	default:
		d, err := officeOpenDeck(p)
		if err != nil {
			return nil, err
		}
		m := &OfficeModel{Type: "pptx"}
		for i := range d.slides {
			s, err := d.slide(i + 1)
			if err != nil {
				return nil, err
			}
			m.Slides = append(m.Slides, s.model(i+1))
		}
		return m, nil
	}
}
func OfficeEdit(b []byte, name string, ops []OfficeOp) ([]byte, error) {
	kind, err := OfficeKind(name)
	if err != nil {
		return nil, err
	}
	p, err := OfficeOpen(b)
	if err != nil {
		return nil, err
	}
	var st *officeIds
	if kind == "docx" {
		w, err := officeOpenWord(p)
		if err != nil {
			return nil, err
		}
		st = officeNewIDs(w.ps)
	}
	for i, op := range ops {
		n := len(p.Log[officeDocPart])
		if err := officeApply(p, kind, op, st); err != nil {
			return nil, fmt.Errorf("edit %d (%s): %v", i, op.Op, err)
		}
		if st != nil {
			if !st.updated {
				for _, ss := range p.Log[officeDocPart][n:] {
					st.update(ss)
				}
			}
			st.updated = false
		}
	}
	var out bytes.Buffer
	if err := p.Save(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func officeApply(p *OfficePackage, kind string, op OfficeOp, st *officeIds) error {
	types, ok := officeOpTypes[op.Op]
	if !ok {
		return fmt.Errorf("unknown op %q; ops are replace_text, insert_paragraph_after, delete_paragraph, set_cell_text (docx), set_cell, set_formula, clear_cell, append_table_row (xlsx), replace_text, replace_shape_text (pptx)", op.Op)
	}
	if !strings.Contains(types, kind) {
		return fmt.Errorf("op %s applies to %s files, not %s", op.Op, types, kind)
	}
	if op.Op == "replace_text" && op.Find == "" {
		return fmt.Errorf("replace_text needs a non-empty \"find\"")
	}
	switch kind {
	case "docx":
		w, err := officeOpenWord(p)
		if err != nil {
			return err
		}
		w.ids = st
		switch op.Op {
		case "replace_text":
			return w.replaceText(op)
		case "insert_paragraph_after":
			return w.insertParagraphAfter(op)
		case "delete_paragraph":
			return w.deleteParagraph(op)
		case "set_cell_text":
			return w.setCellText(op)
		}
	case "xlsx":
		bk, err := officeOpenBook(p)
		if err != nil {
			return err
		}
		if op.Op == "append_table_row" {
			return bk.appendTableRow(op)
		}
		if op.Cell == "" {
			return fmt.Errorf("%s needs \"cell\", for example \"B12\"", op.Op)
		}
		return bk.setCell(op)
	case "pptx":
		d, err := officeOpenDeck(p)
		if err != nil {
			return err
		}
		if op.Op == "replace_shape_text" {
			return d.replaceShapeText(op)
		}
		return d.replaceText(op)
	}
	return nil
}

func (a *Archive) documentBytes(ctx context.Context, id uuid.UUID) (*File, []byte, error) {
	f, rc, err := a.OpenFile(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	return f, b, err
}

func (a *Archive) ReadDocument(ctx context.Context, id uuid.UUID) (*OfficeModel, error) {
	f, b, err := a.documentBytes(ctx, id)
	if err != nil {
		return nil, err
	}
	m, err := OfficeRead(b, f.Name)
	if err != nil {
		return nil, fmt.Errorf("archive.read: %v: %w", err, httpErrBadRequest)
	}
	return m, nil
}

// ViewDocument is a document as OfficeView reads it, for the canvas.
func (a *Archive) ViewDocument(ctx context.Context, id uuid.UUID) (*OfficeModel, error) {
	f, b, err := a.documentBytes(ctx, id)
	if err != nil {
		return nil, err
	}
	m, err := OfficeView(b, f.Name)
	if err != nil {
		return nil, fmt.Errorf("archive.read: %v: %w", err, httpErrBadRequest)
	}
	return m, nil
}

func (a *Archive) EditDocument(ctx context.Context, id uuid.UUID, ops []OfficeOp) (*File, error) {
	nf, err := coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		return a.editDocument(ctx, id, ops)
	})
	if err != nil {
		return nil, err
	}
	return a.fileReady(ctx, nf)
}

func (a *Archive) editDocument(ctx context.Context, id uuid.UUID, ops []OfficeOp) (*File, error) {
	if err := a.fileMine(ctx, id, "edit"); err != nil {
		return nil, err
	}
	f, err := fileOf(a.st.db(ctx).LockFile(ctx, id))
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
	}
	if err == nil {
		err = f.blob()
	}
	if err != nil {
		return nil, err
	}
	rc, err := a.blobs.open(ctx, blobKey(f.Hash), f.Size)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, err
	}
	out, err := OfficeEdit(b, f.Name, ops)
	if err != nil {
		return nil, fmt.Errorf("archive.edit: %v: %w", err, httpErrBadRequest)
	}
	hash, err := a.blobPut(ctx, out)
	if err != nil {
		return nil, err
	}
	return fileOf(a.st.db(ctx).ReplaceFile(ctx, id, hash, int64(len(out))))
}

const officeDocPart = "word/document.xml"

var officeWNS = []string{"http://schemas.openxmlformats.org/wordprocessingml/2006/main", "http://purl.oclc.org/ooxml/wordprocessingml/main"}

type officeWord struct {
	ids  *officeIds
	pkg  *OfficePackage
	b    []byte
	doc  *OfficeNode
	w    string
	tn   officeTextNames
	ps   []*OfficeNode
	tbls []*OfficeNode
}

func officeOpenWord(p *OfficePackage) (*officeWord, error) {
	b, err := p.Part(officeDocPart)
	if err != nil {
		return nil, err
	}
	doc, err := OfficeParse(b)
	if err != nil {
		return nil, err
	}
	w, ok := OfficePrefixOf(doc, officeWNS...)
	if !ok {
		return nil, fmt.Errorf("%s does not declare the WordprocessingML namespace", officeDocPart)
	}
	return &officeWord{nil, p, b, doc, w, officeTextNames{w, "p", "r", "t", true}, doc.Find(w, "p"), doc.Find(w, "tbl")}, nil
}

// own returns the local elements under n whose nearest parent element is n,
// skipping those inside a nested one: a table's own rows, a row's own cells.
func (x *officeWord) own(n *OfficeNode, local, parent string) []*OfficeNode {
	var out []*OfficeNode
	for _, k := range n.Find(x.w, local) {
		if k.Nearest(x.w, parent) == n {
			out = append(out, k)
		}
	}
	return out
}
func (x *officeWord) rows(tbl *OfficeNode) []*OfficeNode { return x.own(tbl, "tr", "tbl") }
func (x *officeWord) cells(tr *OfficeNode) []*OfficeNode { return x.own(tr, "tc", "tr") }
func (x *officeWord) model() *OfficeModel {
	m := &OfficeModel{Type: "docx"}
	tid := map[*OfficeNode]int{}
	for i, t := range x.tbls {
		tid[t] = i
	}
	for i, p := range x.ps {
		mp := OfficeParagraph{ID: "p" + strconv.Itoa(i), Text: x.tn.text(p)}
		if r := x.tn.runs(p); len(r) > 1 {
			mp.Runs = r
		}
		if ppr := p.Child(x.w, "pPr"); ppr != nil {
			if st := ppr.Child(x.w, "pStyle"); st != nil {
				mp.Style = st.Attrv(x.w + ":val")
			}
			if np := ppr.Child(x.w, "numPr"); np != nil {
				lvl, id := "0", ""
				if n := np.Child(x.w, "ilvl"); n != nil {
					lvl = n.Attrv(x.w + ":val")
				}
				if n := np.Child(x.w, "numId"); n != nil {
					id = n.Attrv(x.w + ":val")
				}
				mp.List = "list " + id + " level " + lvl
			}
		}
		if tc := p.Nearest(x.w, "tc"); tc != nil {
			tr := tc.Nearest(x.w, "tr")
			tbl := tr.Nearest(x.w, "tbl")
			mp.In = fmt.Sprintf("t%d/r%d/c%d", tid[tbl], officeIndex(x.rows(tbl), tr), officeIndex(x.cells(tr), tc))
		}
		if p.Nearest(x.w, "txbxContent") != nil {
			mp.In = "textbox"
		}
		m.Paragraphs = append(m.Paragraphs, mp)
	}
	for i, t := range x.tbls {
		mt := OfficeTable{ID: "t" + strconv.Itoa(i)}
		for _, r := range x.rows(t) {
			var row []string
			for _, c := range x.cells(r) {
				var parts []string
				for _, p := range x.own(c, "p", "tc") {
					parts = append(parts, x.tn.text(p))
				}
				row = append(row, strings.Join(parts, "\n"))
			}
			mt.Rows = append(mt.Rows, row)
		}
		m.Tables = append(m.Tables, mt)
	}
	return m
}
func officeIndex(ns []*OfficeNode, n *OfficeNode) int {
	for i, x := range ns {
		if x == n {
			return i
		}
	}
	return -1
}

type officeIds struct {
	start, after []int
	updated      bool
}

func officeNewIDs(ps []*OfficeNode) *officeIds {
	st := &officeIds{}
	for _, p := range ps {
		st.start = append(st.start, p.Start)
		st.after = append(st.after, -1)
	}
	return st
}
func officeMapOffset(o int, ss []OfficeSplice) int {
	if o < 0 {
		return -1
	}
	n := o
	for _, s := range ss {
		if s.Start <= o && o < s.End {
			return -1
		}
		if s.End <= o {
			n += len(s.New) - (s.End - s.Start)
		}
	}
	return n
}
func (st *officeIds) update(ss []OfficeSplice) {
	for i := range st.start {
		st.start[i] = officeMapOffset(st.start[i], ss)
		st.after[i] = officeMapOffset(st.after[i], ss)
	}
}
func (x *officeWord) at(off int) *OfficeNode {
	for _, p := range x.ps {
		if p.Start == off {
			return p
		}
	}
	return nil
}
func (x *officeWord) para(id string) (*OfficeNode, error) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, "p"))
	if err != nil || !strings.HasPrefix(id, "p") || i < 0 || i >= len(x.ids.start) {
		return nil, fmt.Errorf("paragraph %q does not exist; ids are p0..p%d as returned by the model", id, len(x.ids.start)-1)
	}
	p := x.at(x.ids.start[i])
	if p == nil {
		return nil, fmt.Errorf("paragraph %s was deleted or replaced by an earlier edit in this call", id)
	}
	return p, nil
}
func (x *officeWord) replaceText(e OfficeOp) error {
	var target *OfficeNode
	if e.Para != "" {
		p, err := x.para(e.Para)
		if err != nil {
			return err
		}
		if !strings.Contains(x.tn.text(p), e.Find) {
			return fmt.Errorf("paragraph %s does not contain %q; its text is %q", e.Para, e.Find, x.tn.text(p))
		}
		target = p
	} else {
		var hits []string
		for _, p := range x.ps {
			if strings.Contains(x.tn.text(p), e.Find) {
				id := "new paragraph"
				for k, off := range x.ids.start {
					if off == p.Start {
						id = "p" + strconv.Itoa(k)
					}
				}
				hits = append(hits, id)
				target = p
			}
		}
		if len(hits) == 0 {
			return fmt.Errorf("text %q not found in any paragraph (text inside a single paragraph only; tabs and line breaks are not part of the text)", e.Find)
		}
		if len(hits) > 1 && !e.All {
			return fmt.Errorf("text %q occurs in %d paragraphs (%s); pass \"para\" or set \"all\": true", e.Find, len(hits), strings.Join(hits, ", "))
		}
		if e.All {
			return x.replaceAll(e.Find, e.Replace)
		}
	}
	ss, err := x.tn.replace(x.b, target, e.Find, e.Replace)
	if err != nil {
		return err
	}
	return x.pkg.Splice(officeDocPart, ss)
}
func (x *officeWord) replaceAll(find, repl string) error {
	if strings.Contains(repl, find) {
		return fmt.Errorf("\"all\" needs a replacement that does not contain the search text")
	}
	for n := 0; n < 10000; n++ {
		y, err := officeOpenWord(x.pkg)
		if err != nil {
			return err
		}
		var ss []OfficeSplice
		for _, p := range y.ps {
			if strings.Contains(y.tn.text(p), find) {
				s, err := y.tn.replace(y.b, p, find, repl)
				if err != nil {
					return err
				}
				ss = append(ss, s...)
			}
		}
		if len(ss) == 0 {
			return nil
		}
		if err := x.pkg.Splice(officeDocPart, ss); err != nil {
			return err
		}
	}
	return nil
}
func (x *officeWord) newPara(pPr, rPr, text string) string {
	w := x.w
	var b strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			b.WriteString("<" + w + ":br/>")
		}
		b.WriteString("<" + w + `:t xml:space="preserve">` + OfficeEsc(line) + "</" + w + ":t>")
	}
	return "<" + w + ":p>" + pPr + "<" + w + ":r>" + rPr + b.String() + "</" + w + ":r></" + w + ":p>"
}
func (x *officeWord) insertParagraphAfter(e OfficeOp) error {
	p, err := x.para(e.Para)
	if err != nil {
		return err
	}
	i, _ := strconv.Atoi(e.Para[1:])
	anchor := p
	if a := x.at(x.ids.after[i]); a != nil {
		anchor = a
	}
	pPr, rPr := x.likeParagraph(p)
	if e.Style != "" {
		pPr = "<" + x.w + ":pPr><" + x.w + `:pStyle ` + x.w + `:val="` + OfficeEscAttr(e.Style) + `"/></` + x.w + ":pPr>"
	}
	np := x.newPara(pPr, rPr, e.Text)
	if err := x.pkg.Splice(officeDocPart, []OfficeSplice{{anchor.End, anchor.End, []byte(np)}}); err != nil {
		return err
	}
	x.ids.update([]OfficeSplice{{anchor.End, anchor.End, []byte(np)}})
	x.ids.after[i] = anchor.End
	x.ids.updated = true
	return nil
}
func (x *officeWord) likeParagraph(p *OfficeNode) (string, string) {
	var pPr, rPr string
	if n := p.Child(x.w, "pPr"); n != nil {
		var drop []OfficeSplice
		for _, k := range n.Kids {
			if k.Local == "sectPr" || k.Local == "pPrChange" || k.Local == "rPr" {
				drop = append(drop, OfficeSplice{k.Start - n.Start, k.End - n.Start, nil})
			}
		}
		b, _ := OfficeApply(x.b[n.Start:n.End], drop)
		pPr = string(b)
	}
	if rs := x.own(p, "r", "p"); len(rs) > 0 {
		if n := rs[0].Child(x.w, "rPr"); n != nil {
			var drop []OfficeSplice
			for _, k := range n.Kids {
				if k.Local == "ins" || k.Local == "del" || k.Local == "rPrChange" {
					drop = append(drop, OfficeSplice{k.Start - n.Start, k.End - n.Start, nil})
				}
			}
			b, _ := OfficeApply(x.b[n.Start:n.End], drop)
			rPr = string(b)
		}
	}
	return pPr, rPr
}
func (x *officeWord) deleteParagraph(e OfficeOp) error {
	p, err := x.para(e.Para)
	if err != nil {
		return err
	}
	var sib []*OfficeNode
	for _, k := range p.Parent.Kids {
		if k.Local == "p" || k.Local == "tbl" {
			sib = append(sib, k)
		}
	}
	if len(sib) == 1 {
		return fmt.Errorf("paragraph %s is the only paragraph in its cell or container and cannot be deleted; set its text instead", e.Para)
	}
	if ppr := p.Child(x.w, "pPr"); ppr != nil && ppr.Child(x.w, "sectPr") != nil {
		return fmt.Errorf("paragraph %s ends a section (it holds a section break) and cannot be deleted", e.Para)
	}
	return x.pkg.Splice(officeDocPart, []OfficeSplice{{p.Start, p.End, nil}})
}

var officeMarkers = map[string]bool{"bookmarkStart": true, "bookmarkEnd": true, "commentRangeStart": true, "commentRangeEnd": true, "permStart": true, "permEnd": true}

func (x *officeWord) setCellText(e OfficeOp) error {
	if e.Table == nil || e.Row == nil || e.Col == nil {
		return fmt.Errorf("set_cell_text needs table, row and col (0-based)")
	}
	if *e.Table < 0 || *e.Table >= len(x.tbls) {
		return fmt.Errorf("table %d does not exist; the document has %d tables", *e.Table, len(x.tbls))
	}
	rows := x.rows(x.tbls[*e.Table])
	if *e.Row < 0 || *e.Row >= len(rows) {
		return fmt.Errorf("row %d does not exist; table %d has %d rows", *e.Row, *e.Table, len(rows))
	}
	cells := x.cells(rows[*e.Row])
	if *e.Col < 0 || *e.Col >= len(cells) {
		return fmt.Errorf("col %d does not exist; row %d has %d cells (merged cells count once)", *e.Col, *e.Row, len(cells))
	}
	tc := cells[*e.Col]
	var pPr, rPr, keep string
	if p := tc.Child(x.w, "p"); p != nil {
		if n := p.Child(x.w, "pPr"); n != nil {
			pPr = officeRaw(x.b, n)
		}
		if r := p.Find(x.w, "r"); len(r) > 0 {
			rPr = officeRaw(x.b, r[0].Child(x.w, "rPr"))
		}
	}
	var walk func(*OfficeNode)
	walk = func(n *OfficeNode) {
		for _, k := range n.Kids {
			if k.Prefix == x.w && officeMarkers[k.Local] {
				keep += officeRaw(x.b, k)
			} else {
				walk(k)
			}
		}
	}
	start := tc.InStart
	if tcPr := tc.Child(x.w, "tcPr"); tcPr != nil {
		start = tcPr.End
	}
	for _, k := range tc.Kids {
		if k.Start >= start {
			walk(k)
		}
	}
	para := x.newPara(pPr, rPr, e.Text)
	if keep != "" {
		i := strings.Index(para, "<"+x.w+":r>")
		para = para[:i] + keep + para[i:]
	}
	return x.pkg.Splice(officeDocPart, []OfficeSplice{{start, tc.InEnd, []byte(para)}})
}

type officeTextNames struct {
	p, para, run, t string
	preserve        bool
}
type officeSeg struct {
	t, run *OfficeNode
	text   string
	brk    bool
}

func (tn officeTextNames) segments(para *OfficeNode) []officeSeg {
	var out []officeSeg
	var walk func(*OfficeNode)
	walk = func(n *OfficeNode) {
		for _, k := range n.Kids {
			if k.Prefix == tn.p && k.Local == tn.para {
				continue
			}
			if k.Prefix == tn.p {
				switch {
				case k.Local == tn.t:
					out = append(out, officeSeg{k, k.Parent, k.Text, false})
					continue
				case (k.Local == "br" || k.Local == "cr") && (k.Parent.Local == tn.run || k.Parent == para):
					out = append(out, officeSeg{k, k.Parent, "\n", true})
					continue
				case k.Local == "tab" && k.Parent.Local == tn.run:
					out = append(out, officeSeg{k, k.Parent, "\t", true})
					continue
				}
			}
			walk(k)
		}
	}
	walk(para)
	return out
}
func (tn officeTextNames) text(para *OfficeNode) string {
	var b strings.Builder
	for _, s := range tn.segments(para) {
		b.WriteString(s.text)
	}
	return b.String()
}
func (tn officeTextNames) runs(para *OfficeNode) []string {
	var out []string
	var last *OfficeNode
	for _, s := range tn.segments(para) {
		if s.run == last && len(out) > 0 {
			out[len(out)-1] += s.text
			continue
		}
		out = append(out, s.text)
		last = s.run
	}
	return out
}
func (tn officeTextNames) tElem(t *OfficeNode, text string) []byte {
	name := OfficeQ(t.Prefix, t.Local)
	if !tn.preserve {
		return []byte("<" + name + ">" + OfficeEsc(text) + "</" + name + ">")
	}
	var out strings.Builder
	start := 0
	flush := func(end int) {
		if end > start {
			out.WriteString("<" + name + ` xml:space="preserve">` + OfficeEsc(text[start:end]) + "</" + name + ">")
		}
	}
	for i, c := range text {
		if c == '\n' || c == '\t' {
			flush(i)
			if c == '\n' {
				out.WriteString("<" + OfficeQ(t.Prefix, "br") + "/>")
			} else {
				out.WriteString("<" + OfficeQ(t.Prefix, "tab") + "/>")
			}
			start = i + 1
		}
	}
	flush(len(text))
	return []byte(out.String())
}
func (tn officeTextNames) replace(b []byte, para *OfficeNode, find, repl string) ([]OfficeSplice, error) {
	segs := tn.segments(para)
	var full strings.Builder
	for _, s := range segs {
		full.WriteString(s.text)
	}
	i := strings.Index(full.String(), find)
	if find == "" || i < 0 {
		return nil, fmt.Errorf("text %q not found in paragraph", find)
	}
	end := i + len(find)
	var a, z int
	cum := 0
	starts := make([]int, len(segs))
	for k, s := range segs {
		starts[k] = cum
		if i >= cum && i < cum+len(s.text) {
			a = k
		}
		if end > cum && end <= cum+len(s.text) {
			z = k
		}
		cum += len(s.text)
	}
	if !tn.preserve && strings.ContainsAny(repl, "\n\t") {
		return nil, fmt.Errorf("the replacement may not contain line breaks or tabs in slides; use replace_shape_text for several paragraphs")
	}
	if segs[a].brk || segs[z].brk {
		return nil, fmt.Errorf("the text to find may not start or end with a tab or line break; include the text next to it")
	}
	prefix := segs[a].text[:i-starts[a]]
	suffix := segs[z].text[end-starts[z]:]
	if a == z {
		return []OfficeSplice{{segs[a].t.Start, segs[a].t.End, tn.tElem(segs[a].t, prefix+repl+suffix)}}, nil
	}
	ss := []OfficeSplice{{segs[a].t.Start, segs[a].t.End, tn.tElem(segs[a].t, prefix+repl)}}
	gone := map[*OfficeNode]bool{}
	for k := a + 1; k <= z; k++ {
		if k == z && suffix != "" {
			ss = append(ss, OfficeSplice{segs[z].t.Start, segs[z].t.End, tn.tElem(segs[z].t, suffix)})
		} else {
			gone[segs[k].t] = true
		}
	}
	runDone := map[*OfficeNode]bool{}
	for k := a + 1; k <= z; k++ {
		s := segs[k]
		if !gone[s.t] || runDone[s.run] {
			continue
		}
		whole := s.run != segs[a].run
		for _, c := range s.run.Kids {
			if c.Local != "rPr" && !gone[c] {
				whole = false
			}
		}
		if whole {
			runDone[s.run] = true
			ss = append(ss, OfficeSplice{s.run.Start, s.run.End, nil})
		} else {
			ss = append(ss, OfficeSplice{s.t.Start, s.t.End, nil})
		}
	}
	return ss, nil
}
func officeRaw(b []byte, n *OfficeNode) string {
	if n == nil {
		return ""
	}
	return string(b[n.Start:n.End])
}

type officeXtable struct {
	part, name     string
	ref            string
	columns        []string
	totals         bool
	b              []byte
	root, autoFilt *OfficeNode
}

func (s *officeSheet) tables() []officeXtable {
	rels, _ := s.bk.pkg.Rels(s.part)
	var out []officeXtable
	for _, r := range rels {
		if !strings.HasSuffix(r.Type, "/table") {
			continue
		}
		b, err := s.bk.pkg.Part(r.Target)
		if err != nil {
			continue
		}
		doc, err := OfficeParse(b)
		if err != nil {
			continue
		}
		x, _ := OfficePrefixOf(doc, officeXNS...)
		root := OfficeRoot(doc)
		t := officeXtable{part: r.Target, name: root.Attrv("displayName"), ref: root.Attrv("ref"), b: b, root: root, autoFilt: root.Child(x, "autoFilter")}
		if t.name == "" {
			t.name = root.Attrv("name")
		}
		n, _ := strconv.Atoi(root.Attrv("totalsRowCount"))
		t.totals = n > 0
		for _, c := range root.Find(x, "tableColumn") {
			t.columns = append(t.columns, c.Attrv("name"))
		}
		out = append(out, t)
	}
	return out
}

var officeRefAttr = regexp.MustCompile(`(\sref=")[^"]*(")`)

func (bk *officeBook) appendTableRow(e OfficeOp) error {
	s, err := bk.sheet(e.Sheet)
	if err != nil {
		return err
	}
	var t *officeXtable
	var names []string
	for _, x := range s.tables() {
		names = append(names, x.name)
		if strings.EqualFold(x.name, e.TableName) {
			x := x
			t = &x
		}
	}
	if t == nil {
		return fmt.Errorf("table %q not found on sheet %q; its tables are %q", e.TableName, s.name, names)
	}
	if t.totals {
		return fmt.Errorf("table %s has a totals row; appending rows to it is not supported", t.name)
	}
	c1, r1, c2, r2, err := officeRangeOf(t.ref)
	if err != nil {
		return err
	}
	if len(e.Values) != c2-c1+1 {
		return fmt.Errorf("table %s has %d columns %q; values has %d items", t.name, c2-c1+1, t.columns, len(e.Values))
	}
	row := r2 + 1
	m := s.model()
	for _, c := range m.Cells {
		cc, rr, _ := OfficeParseRef(c.Ref)
		if rr == row && cc >= c1 && cc <= c2 {
			return fmt.Errorf("the row below table %s (row %d) is not empty (%s = %q); move that content first", t.name, row, c.Ref, c.Value)
		}
	}
	for i, v := range e.Values {
		ref := OfficeColName(c1+i) + strconv.Itoa(row)
		above := OfficeColName(c1+i) + strconv.Itoa(r2)
		op := OfficeOp{Op: "set_cell", Sheet: s.name, Cell: ref, Value: v}
		if str, ok := v.(string); ok && strings.HasPrefix(str, "=") {
			op = OfficeOp{Op: "set_formula", Sheet: s.name, Cell: ref, Formula: str}
		}
		bk.styleFrom = above
		err := bk.setCell(op)
		bk.styleFrom = ""
		if err != nil {
			return err
		}
	}
	nref := fmt.Sprintf("%s%d:%s%d", OfficeColName(c1), r1, OfficeColName(c2), row)
	ss := []OfficeSplice{{t.root.Start, t.root.InStart, []byte(officeRefAttr.ReplaceAllString(string(t.b[t.root.Start:t.root.InStart]), "${1}"+nref+"${2}"))}}
	if a := t.autoFilt; a != nil {
		ss = append(ss, OfficeSplice{a.Start, a.InStart, []byte(officeRefAttr.ReplaceAllString(string(t.b[a.Start:a.InStart]), "${1}"+nref+"${2}"))})
	}
	return bk.pkg.Splice(t.part, ss)
}

var officeXNS = []string{"http://schemas.openxmlformats.org/spreadsheetml/2006/main", "http://purl.oclc.org/ooxml/spreadsheetml/main"}

type officeBook struct {
	pkg       *OfficePackage
	wbPart    string
	names     []string
	parts     []string
	shared    []string
	styleFrom string
}

func officeOpenBook(p *OfficePackage) (*officeBook, error) {
	rels, err := p.Rels("")
	if err != nil {
		return nil, err
	}
	bk := &officeBook{pkg: p}
	for _, r := range rels {
		if strings.HasSuffix(r.Type, "/officeDocument") {
			bk.wbPart = r.Target
		}
	}
	b, err := p.Part(bk.wbPart)
	if err != nil {
		return nil, err
	}
	doc, err := OfficeParse(b)
	if err != nil {
		return nil, err
	}
	x, _ := OfficePrefixOf(doc, officeXNS...)
	wrels, err := p.Rels(bk.wbPart)
	if err != nil {
		return nil, err
	}
	for _, s := range OfficeRoot(doc).Find(x, "sheet") {
		id := ""
		for _, a := range s.Attr {
			if a.Name.Local == "id" && a.Name.Space != "" {
				id = a.Value
			}
		}
		for _, r := range wrels {
			if r.ID == id {
				bk.names = append(bk.names, s.Attrv("name"))
				bk.parts = append(bk.parts, r.Target)
			}
		}
	}
	for _, r := range wrels {
		if strings.HasSuffix(r.Type, "/sharedStrings") {
			if sb, err := p.Part(r.Target); err == nil {
				if sd, err := OfficeParse(sb); err == nil {
					sx, _ := OfficePrefixOf(sd, officeXNS...)
					for _, si := range OfficeRoot(sd).Children(sx, "si") {
						var t strings.Builder
						for _, tt := range si.Find(sx, "t") {
							if tt.Nearest(sx, "rPh") == nil {
								t.WriteString(tt.Text)
							}
						}
						bk.shared = append(bk.shared, t.String())
					}
				}
			}
		}
	}
	return bk, nil
}
func (bk *officeBook) sheet(name string) (*officeSheet, error) {
	i := -1
	for k, n := range bk.names {
		if n == name || (name == "" && k == 0) {
			i = k
		}
	}
	if i < 0 {
		return nil, fmt.Errorf("sheet %q does not exist; sheets are %q", name, bk.names)
	}
	b, err := bk.pkg.Part(bk.parts[i])
	if err != nil {
		return nil, err
	}
	doc, err := OfficeParse(b)
	if err != nil {
		return nil, err
	}
	x, ok := OfficePrefixOf(doc, officeXNS...)
	if !ok {
		return nil, fmt.Errorf("%s is not a worksheet", bk.parts[i])
	}
	s := &officeSheet{bk: bk, name: bk.names[i], part: bk.parts[i], b: b, doc: doc, x: x}
	s.data = OfficeRoot(doc).Child(x, "sheetData")
	if s.data == nil {
		return nil, fmt.Errorf("sheet %q has no sheetData (chart sheet?)", name)
	}
	return s, nil
}

type officeSheet struct {
	bk         *officeBook
	name, part string
	b          []byte
	doc, data  *OfficeNode
	x          string
}
type officeRowAt struct {
	n *OfficeNode
	r int
}

func (s *officeSheet) rows() []officeRowAt {
	var out []officeRowAt
	prev := 0
	for _, r := range s.data.Children(s.x, "row") {
		n := prev + 1
		if v, ok := r.Get("r"); ok {
			n, _ = strconv.Atoi(v)
		}
		out = append(out, officeRowAt{r, n})
		prev = n
	}
	return out
}
func (s *officeSheet) cells(row officeRowAt) []officeRowAt {
	var out []officeRowAt
	prev := 0
	for _, c := range row.n.Children(s.x, "c") {
		n := prev + 1
		if v, ok := c.Get("r"); ok {
			n, _, _ = OfficeParseRef(v)
		}
		out = append(out, officeRowAt{c, n})
		prev = n
	}
	return out
}

var officeRefRe = regexp.MustCompile(`^\$?([A-Za-z]{1,3})\$?([0-9]+)$`)

func OfficeParseRef(ref string) (col, row int, err error) {
	m := officeRefRe.FindStringSubmatch(ref)
	if m == nil {
		return 0, 0, fmt.Errorf("%q is not a cell reference like B12", ref)
	}
	for _, ch := range strings.ToUpper(m[1]) {
		col = col*26 + int(ch-'A'+1)
	}
	row, _ = strconv.Atoi(m[2])
	if col > 16384 || row < 1 || row > 1048576 {
		return 0, 0, fmt.Errorf("%q is outside the sheet", ref)
	}
	return col, row, nil
}
func OfficeColName(c int) string {
	s := ""
	for c > 0 {
		c--
		s = string(rune('A'+c%26)) + s
		c /= 26
	}
	return s
}
func officeRangeOf(ref string) (c1, r1, c2, r2 int, err error) {
	a, b, ok := strings.Cut(ref, ":")
	if c1, r1, err = OfficeParseRef(a); err != nil {
		return
	}
	c2, r2 = c1, r1
	if ok {
		c2, r2, err = OfficeParseRef(b)
	}
	return
}
func (s *officeSheet) value(c *OfficeNode) string {
	t := c.Attrv("t")
	if t == "inlineStr" {
		var b strings.Builder
		if is := c.Child(s.x, "is"); is != nil {
			for _, tt := range is.Find(s.x, "t") {
				b.WriteString(tt.Text)
			}
		}
		return b.String()
	}
	v := c.Child(s.x, "v")
	if v == nil {
		return ""
	}
	switch t {
	case "s":
		if i, err := strconv.Atoi(strings.TrimSpace(v.Text)); err == nil && i < len(s.bk.shared) {
			return s.bk.shared[i]
		}
	case "b":
		if v.Text == "1" {
			return "TRUE"
		}
		return "FALSE"
	}
	return v.Text
}
func (s *officeSheet) sharedMasters() map[string]*OfficeNode {
	m := map[string]*OfficeNode{}
	for _, f := range s.data.Find(s.x, "f") {
		if f.Attrv("t") == "shared" && f.Attrv("ref") != "" {
			m[f.Attrv("si")] = f
		}
	}
	return m
}
func (s *officeSheet) formula(c *OfficeNode, masters map[string]*OfficeNode) string {
	f := c.Child(s.x, "f")
	if f == nil {
		return ""
	}
	if f.Attrv("t") == "shared" && f.Attrv("ref") == "" {
		if m := masters[f.Attrv("si")]; m != nil {
			mc, mr, _ := OfficeParseRef(m.Parent.Attrv("r"))
			cc, cr, _ := OfficeParseRef(c.Attrv("r"))
			return OfficeShiftFormula(m.Text, cr-mr, cc-mc)
		}
	}
	return f.Text
}
func (s *officeSheet) model() OfficeSheet {
	out := OfficeSheet{Name: s.name, Cells: []OfficeCell{}}
	if d := OfficeRoot(s.doc).Child(s.x, "dimension"); d != nil {
		out.Dimension = d.Attrv("ref")
	}
	for _, mc := range OfficeRoot(s.doc).Find(s.x, "mergeCell") {
		out.Merged = append(out.Merged, mc.Attrv("ref"))
	}
	for _, t := range s.tables() {
		out.Tables = append(out.Tables, OfficeXTable{t.name, t.ref, t.columns})
	}
	masters := s.sharedMasters()
	for _, r := range s.rows() {
		for _, c := range s.cells(r) {
			cell := OfficeCell{Ref: OfficeColName(c.r) + strconv.Itoa(r.r), Value: s.value(c.n), Formula: s.formula(c.n, masters)}
			if cell.Value != "" || cell.Formula != "" {
				out.Cells = append(out.Cells, cell)
			}
		}
	}
	return out
}

var officeCellRefInFormula = regexp.MustCompile(`\$?[A-Za-z]{1,3}\$?[0-9]+`)

func OfficeShiftFormula(f string, dr, dc int) string {
	var out strings.Builder
	i := 0
	for i < len(f) {
		c := f[i]
		if c == '"' || c == '\'' {
			j := i + 1
			for j < len(f) {
				if f[j] == c {
					if j+1 < len(f) && f[j+1] == c {
						j += 2
						continue
					}
					break
				}
				j++
			}
			j = min(j+1, len(f))
			out.WriteString(f[i:j])
			i = j
			continue
		}
		if c == '[' {
			depth, j := 0, i
			for ; j < len(f); j++ {
				if f[j] == '[' {
					depth++
				} else if f[j] == ']' {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			j = min(j+1, len(f))
			out.WriteString(f[i:j])
			i = j
			continue
		}
		j := i
		for j < len(f) && f[j] != '"' && f[j] != '\'' && f[j] != '[' {
			j++
		}
		seg := f[i:j]
		last := 0
		for _, m := range officeCellRefInFormula.FindAllStringIndex(seg, -1) {
			a, b := m[0], m[1]
			before := byte(0)
			if a > 0 {
				before = seg[a-1]
			}
			after := byte(0)
			if b < len(seg) {
				after = seg[b]
			}
			if officeIsIdent(before) || officeIsIdent(after) || after == '(' || after == '!' {
				continue
			}
			out.WriteString(seg[last:a])
			out.WriteString(officeShiftRef(seg[a:b], dr, dc))
			last = b
		}
		out.WriteString(seg[last:])
		i = j
	}
	return out.String()
}
func officeIsIdent(c byte) bool {
	return c == '_' || c == '.' || c == '$' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}
func officeShiftRef(ref string, dr, dc int) string {
	m := regexp.MustCompile(`^(\$?)([A-Za-z]{1,3})(\$?)([0-9]+)$`).FindStringSubmatch(ref)
	col, row, err := OfficeParseRef(ref)
	if err != nil {
		return ref
	}
	if m[1] == "" {
		col += dc
	}
	if m[3] == "" {
		row += dr
	}
	if col < 1 || row < 1 {
		return "#REF!"
	}
	return m[1] + OfficeColName(col) + m[3] + strconv.Itoa(row)
}
func (s *officeSheet) setCell(ref, typ, inner string) error {
	col, rown, err := OfficeParseRef(ref)
	if err != nil {
		return err
	}
	ref = OfficeColName(col) + strconv.Itoa(rown)
	for _, mc := range OfficeRoot(s.doc).Find(s.x, "mergeCell") {
		c1, r1, c2, r2, err := officeRangeOf(mc.Attrv("ref"))
		if err == nil && col >= c1 && col <= c2 && rown >= r1 && rown <= r2 && (col != c1 || rown != r1) {
			return fmt.Errorf("%s is hidden inside merged range %s; write to its top-left cell %s%d instead", ref, mc.Attrv("ref"), OfficeColName(c1), r1)
		}
	}
	for _, f := range s.data.Find(s.x, "f") {
		if f.Attrv("t") != "array" || f.Attrv("ref") == "" {
			continue
		}
		c1, r1, c2, r2, err := officeRangeOf(f.Attrv("ref"))
		if err == nil && col >= c1 && col <= c2 && rown >= r1 && rown <= r2 && (c1 != c2 || r1 != r2) {
			return fmt.Errorf("%s is part of the array formula over %s, which cannot be edited cell by cell", ref, f.Attrv("ref"))
		}
	}
	x := s.x
	c := "<" + OfficeQ(x, "c") + ` r="` + ref + `"`
	var ss []OfficeSplice
	var old *OfficeNode
	var row *officeRowAt
	for _, r := range s.rows() {
		if r.r == rown {
			r := r
			row = &r
		}
	}
	if row != nil {
		for _, cl := range s.cells(*row) {
			if cl.r == col {
				old = cl.n
			}
		}
	}
	if old != nil {
		if st, ok := old.Get("s"); ok {
			c += ` s="` + st + `"`
		}
	} else if st := s.styleOf(s.bk.styleFrom); st != "" {
		c += ` s="` + st + `"`
	} else if row != nil {
		if st, ok := row.n.Get("s"); ok && row.n.Attrv("customFormat") == "1" {
			c += ` s="` + st + `"`
		}
	}
	if typ != "" {
		c += ` t="` + typ + `"`
	}
	if inner == "" {
		c += "/>"
	} else {
		c += ">" + inner + "</" + OfficeQ(x, "c") + ">"
	}
	switch {
	case old != nil:
		ss = append(ss, OfficeSplice{old.Start, old.End, []byte(c)})
		if f := old.Child(x, "f"); f != nil {
			if f.Attrv("t") == "shared" && f.Attrv("ref") != "" {
				ss = append(ss, s.unshare(old, f)...)
			}
			if err := s.dropCalcChain(); err != nil {
				return err
			}
		}
	case row != nil:
		at := row.n.InEnd
		for _, cl := range s.cells(*row) {
			if cl.r > col {
				at = cl.n.Start
				break
			}
		}
		if row.n.SelfClosing() {
			tag := strings.TrimSuffix(strings.TrimRight(strings.TrimSuffix(officeRaw(s.b, row.n), "/>"), " "), "/")
			ss = append(ss, OfficeSplice{row.n.Start, row.n.End, []byte(officeDropSpans(tag) + ">" + c + "</" + OfficeQ(x, "row") + ">")})
		} else {
			ss = append(ss, OfficeSplice{at, at, []byte(c)})
			if _, ok := row.n.Get("spans"); ok {
				ss = append(ss, OfficeSplice{row.n.Start, row.n.InStart, []byte(officeDropSpans(string(s.b[row.n.Start:row.n.InStart])))})
			}
		}
	default:
		nr := "<" + OfficeQ(x, "row") + ` r="` + strconv.Itoa(rown) + `">` + c + "</" + OfficeQ(x, "row") + ">"
		if s.data.SelfClosing() {
			ss = append(ss, OfficeSplice{s.data.Start, s.data.End, []byte("<" + OfficeQ(x, "sheetData") + ">" + nr + "</" + OfficeQ(x, "sheetData") + ">")})
		} else {
			at := s.data.InEnd
			for _, r := range s.rows() {
				if r.r > rown {
					at = r.n.Start
					break
				}
			}
			ss = append(ss, OfficeSplice{at, at, []byte(nr)})
		}
	}
	if d := OfficeRoot(s.doc).Child(x, "dimension"); d != nil {
		c1, r1, c2, r2, err := officeRangeOf(d.Attrv("ref"))
		if err == nil && (col < c1 || col > c2 || rown < r1 || rown > r2) {
			c1, r1, c2, r2 = min(c1, col), min(r1, rown), max(c2, col), max(r2, rown)
			nd := "<" + OfficeQ(x, "dimension") + ` ref="` + OfficeColName(c1) + strconv.Itoa(r1) + ":" + OfficeColName(c2) + strconv.Itoa(r2) + `"/>`
			ss = append(ss, OfficeSplice{d.Start, d.End, []byte(nd)})
		}
	}
	if err := s.bk.pkg.Splice(s.part, ss); err != nil {
		return err
	}
	return s.bk.fullCalcOnLoad()
}
func (s *officeSheet) styleOf(ref string) string {
	if ref == "" {
		return ""
	}
	col, rown, err := OfficeParseRef(ref)
	if err != nil {
		return ""
	}
	for _, r := range s.rows() {
		if r.r == rown {
			for _, c := range s.cells(r) {
				if c.r == col {
					return c.n.Attrv("s")
				}
			}
		}
	}
	return ""
}

var officeSpansRe = regexp.MustCompile(`\s+spans="[^"]*"`)

func officeDropSpans(tag string) string {
	return officeSpansRe.ReplaceAllString(tag, "")
}
func (s *officeSheet) unshare(master, f *OfficeNode) []OfficeSplice {
	var ss []OfficeSplice
	mc, mr, _ := OfficeParseRef(master.Attrv("r"))
	for _, g := range s.data.Find(s.x, "f") {
		if g == f || g.Attrv("t") != "shared" || g.Attrv("si") != f.Attrv("si") {
			continue
		}
		cc, cr, _ := OfficeParseRef(g.Parent.Attrv("r"))
		nf := "<" + OfficeQ(g.Prefix, "f") + ">" + OfficeEsc(OfficeShiftFormula(f.Text, cr-mr, cc-mc)) + "</" + OfficeQ(g.Prefix, "f") + ">"
		ss = append(ss, OfficeSplice{g.Start, g.End, []byte(nf)})
	}
	return ss
}
func (bk *officeBook) dropCalcChain() error {
	rels, err := bk.pkg.Rels(bk.wbPart)
	if err != nil {
		return err
	}
	for _, r := range rels {
		if strings.HasSuffix(r.Type, "/calcChain") && bk.pkg.Has(r.Target) {
			return bk.pkg.DropPart(r.Target, bk.wbPart)
		}
	}
	return nil
}
func (s *officeSheet) dropCalcChain() error {
	return s.bk.dropCalcChain()
}

var officeFullCalcRe = regexp.MustCompile(`\s+fullCalcOnLoad="[^"]*"`)

func (bk *officeBook) fullCalcOnLoad() error {
	b, err := bk.pkg.Part(bk.wbPart)
	if err != nil {
		return err
	}
	doc, err := OfficeParse(b)
	if err != nil {
		return err
	}
	x, _ := OfficePrefixOf(doc, officeXNS...)
	root := OfficeRoot(doc)
	if cp := root.Child(x, "calcPr"); cp != nil {
		if cp.Attrv("fullCalcOnLoad") == "1" || cp.Attrv("fullCalcOnLoad") == "true" {
			return nil
		}
		tag := officeFullCalcRe.ReplaceAllString(string(b[cp.Start:cp.InStart]), "")
		if strings.HasSuffix(tag, "/>") {
			tag = strings.TrimSuffix(tag, "/>") + ` fullCalcOnLoad="1"/>`
		} else {
			tag = strings.TrimSuffix(tag, ">") + ` fullCalcOnLoad="1">`
		}
		return bk.pkg.Splice(bk.wbPart, []OfficeSplice{{cp.Start, cp.InStart, []byte(tag)}})
	}
	at := -1
	for _, k := range root.Kids {
		switch k.Local {
		case "sheets", "functionGroups", "externalReferences", "definedNames":
			at = k.End
		}
	}
	if at < 0 {
		return fmt.Errorf("workbook has no sheets element")
	}
	return bk.pkg.Splice(bk.wbPart, []OfficeSplice{{at, at, []byte("<" + OfficeQ(x, "calcPr") + ` fullCalcOnLoad="1"/>`)}})
}
func (bk *officeBook) setCell(e OfficeOp) error {
	s, err := bk.sheet(e.Sheet)
	if err != nil {
		return err
	}
	x := s.x
	switch e.Op {
	case "set_formula":
		f := strings.TrimPrefix(strings.TrimSpace(e.Formula), "=")
		if f == "" {
			return fmt.Errorf("set_formula needs a formula, for example \"SUM(B2:B9)\"")
		}
		return s.setCell(e.Cell, "", "<"+OfficeQ(x, "f")+">"+OfficeEsc(f)+"</"+OfficeQ(x, "f")+">")
	case "clear_cell":
		return s.setCell(e.Cell, "", "")
	}
	switch v := e.Value.(type) {
	case string:
		return s.setCell(e.Cell, "inlineStr", "<"+OfficeQ(x, "is")+"><"+OfficeQ(x, "t")+` xml:space="preserve">`+OfficeEsc(v)+"</"+OfficeQ(x, "t")+"></"+OfficeQ(x, "is")+">")
	case float64:
		return s.setCell(e.Cell, "", "<"+OfficeQ(x, "v")+">"+strconv.FormatFloat(v, 'g', -1, 64)+"</"+OfficeQ(x, "v")+">")
	case bool:
		b := "0"
		if v {
			b = "1"
		}
		return s.setCell(e.Cell, "b", "<"+OfficeQ(x, "v")+">"+b+"</"+OfficeQ(x, "v")+">")
	case nil:
		return fmt.Errorf("set_cell needs a value: a JSON string, number or boolean (use clear_cell to empty a cell)")
	}
	return fmt.Errorf("set_cell value must be a string, number or boolean, got %T", e.Value)
}

var (
	officeANS = []string{"http://schemas.openxmlformats.org/drawingml/2006/main", "http://purl.oclc.org/ooxml/drawingml/main"}
	officePNS = []string{"http://schemas.openxmlformats.org/presentationml/2006/main", "http://purl.oclc.org/ooxml/presentationml/main"}
)

type officeDeck struct {
	pkg    *OfficePackage
	slides []string
	w, h   int64
}

func officeOpenDeck(p *OfficePackage) (*officeDeck, error) {
	rels, err := p.Rels("")
	if err != nil {
		return nil, err
	}
	pres := ""
	for _, r := range rels {
		if strings.HasSuffix(r.Type, "/officeDocument") {
			pres = r.Target
		}
	}
	b, err := p.Part(pres)
	if err != nil {
		return nil, err
	}
	doc, err := OfficeParse(b)
	if err != nil {
		return nil, err
	}
	px, _ := OfficePrefixOf(doc, officePNS...)
	prels, err := p.Rels(pres)
	if err != nil {
		return nil, err
	}
	d := &officeDeck{pkg: p}
	if sz := OfficeRoot(doc).Child(px, "sldSz"); sz != nil {
		d.w, d.h = officeInt(sz.Attrv("cx")), officeInt(sz.Attrv("cy"))
	}
	for _, s := range OfficeRoot(doc).Find(px, "sldId") {
		for _, a := range s.Attr {
			if a.Name.Local == "id" && a.Name.Space != "" {
				for _, r := range prels {
					if r.ID == a.Value {
						d.slides = append(d.slides, r.Target)
					}
				}
			}
		}
	}
	return d, nil
}

type officeSlide struct {
	hidden bool
	part   string
	b      []byte
	a, p   string
	tn     officeTextNames
	shapes []*OfficeNode
}

func (d *officeDeck) slide(n int) (*officeSlide, error) {
	if n < 1 || n > len(d.slides) {
		return nil, fmt.Errorf("slide %d does not exist; slides are numbered 1..%d", n, len(d.slides))
	}
	part := d.slides[n-1]
	b, err := d.pkg.Part(part)
	if err != nil {
		return nil, err
	}
	doc, err := OfficeParse(b)
	if err != nil {
		return nil, err
	}
	a, _ := OfficePrefixOf(doc, officeANS...)
	p, _ := OfficePrefixOf(doc, officePNS...)
	s := &officeSlide{hidden: OfficeRoot(doc).Attrv("show") == "0", part: part, b: b, a: a, p: p, tn: officeTextNames{a, "p", "r", "t", false}}
	for _, sp := range OfficeRoot(doc).Find(p, "sp") {
		if sp.Child(p, "txBody") != nil {
			s.shapes = append(s.shapes, sp)
		}
	}
	return s, nil
}
func (s *officeSlide) paras(sp *OfficeNode) []*OfficeNode {
	return sp.Child(s.p, "txBody").Children(s.a, "p")
}
func (s *officeSlide) model(n int) OfficeSlide {
	out := OfficeSlide{Number: n, Hidden: s.hidden, Shapes: []OfficeShape{}}
	for i, sp := range s.shapes {
		sh := OfficeShape{Index: i}
		if nv := sp.Child(s.p, "nvSpPr"); nv != nil {
			if c := nv.Child(s.p, "cNvPr"); c != nil {
				sh.Name = c.Attrv("name")
			}
			if nvp := nv.Child(s.p, "nvPr"); nvp != nil {
				if ph := nvp.Child(s.p, "ph"); ph != nil {
					sh.Placeholder = ph.Attrv("type")
					if sh.Placeholder == "" {
						sh.Placeholder = "body"
					}
				}
			}
		}
		for _, p := range s.paras(sp) {
			mp := OfficeParagraph{Text: s.tn.text(p)}
			if r := s.tn.runs(p); len(r) > 1 {
				mp.Runs = r
			}
			sh.Paragraphs = append(sh.Paragraphs, mp)
		}
		out.Shapes = append(out.Shapes, sh)
	}
	return out
}
func (d *officeDeck) shape(e OfficeOp) (*officeSlide, *OfficeNode, error) {
	s, err := d.slide(e.Slide)
	if err != nil {
		return nil, nil, err
	}
	if e.Shape == nil || *e.Shape < 0 || *e.Shape >= len(s.shapes) {
		return nil, nil, fmt.Errorf("shape must be an index 0..%d on slide %d (shapes with text, as listed by the model)", len(s.shapes)-1, e.Slide)
	}
	return s, s.shapes[*e.Shape], nil
}
func (d *officeDeck) replaceShapeText(e OfficeOp) error {
	s, sp, err := d.shape(e)
	if err != nil {
		return err
	}
	ps := s.paras(sp)
	if len(ps) == 0 {
		return fmt.Errorf("shape %d has no paragraphs", *e.Shape)
	}
	var pPr, rPr, end string
	if n := ps[0].Child(s.a, "pPr"); n != nil {
		pPr = officeRaw(s.b, n)
	}
	for _, p := range ps {
		if r := p.Find(s.a, "r"); len(r) > 0 {
			rPr = officeRaw(s.b, r[0].Child(s.a, "rPr"))
			break
		}
	}
	if n := ps[len(ps)-1].Child(s.a, "endParaRPr"); n != nil {
		end = officeRaw(s.b, n)
	}
	a := s.a
	var b strings.Builder
	for _, line := range strings.Split(e.Text, "\n") {
		b.WriteString("<" + OfficeQ(a, "p") + ">" + pPr)
		if line != "" {
			b.WriteString("<" + OfficeQ(a, "r") + ">" + rPr + "<" + OfficeQ(a, "t") + ">" + OfficeEsc(line) + "</" + OfficeQ(a, "t") + "></" + OfficeQ(a, "r") + ">")
		}
		b.WriteString(end + "</" + OfficeQ(a, "p") + ">")
	}
	return d.pkg.Splice(s.part, []OfficeSplice{{ps[0].Start, ps[len(ps)-1].End, []byte(b.String())}})
}
func (d *officeDeck) replaceText(e OfficeOp) error {
	type hit struct {
		s     *officeSlide
		p     *OfficeNode
		where string
	}
	var hits []hit
	for n := 1; n <= len(d.slides); n++ {
		if e.Slide != 0 && n != e.Slide {
			continue
		}
		s, err := d.slide(n)
		if err != nil {
			return err
		}
		for i, sp := range s.shapes {
			if e.Shape != nil && *e.Shape != i {
				continue
			}
			for _, p := range s.paras(sp) {
				if strings.Contains(s.tn.text(p), e.Find) {
					hits = append(hits, hit{s, p, fmt.Sprintf("slide %d shape %d", n, i)})
				}
			}
		}
	}
	if len(hits) == 0 {
		return fmt.Errorf("text %q not found in a single paragraph of any shape (narrow with slide/shape, or use replace_shape_text)", e.Find)
	}
	if len(hits) > 1 && !e.All {
		var w []string
		for _, h := range hits {
			w = append(w, h.where)
		}
		return fmt.Errorf("text %q occurs %d times (%s); pass slide and shape, or set \"all\": true", e.Find, len(hits), strings.Join(w, "; "))
	}
	byPart := map[string][]OfficeSplice{}
	for _, h := range hits {
		ss, err := h.s.tn.replace(h.s.b, h.p, e.Find, e.Replace)
		if err != nil {
			return err
		}
		byPart[h.s.part] = append(byPart[h.s.part], ss...)
	}
	for part, ss := range byPart {
		if err := d.pkg.Splice(part, ss); err != nil {
			return err
		}
	}
	return nil
}

const (
	officeNSW   = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	officeNSS   = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	officeNSP   = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	officeRel   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	officeXML   = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
	officeCTRel = `<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>`
)

func officeRels(rels ...string) string {
	var b strings.Builder
	b.WriteString(officeXML + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 0; i+1 < len(rels); i += 2 {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="%s%s" Target="%s"/>`, i/2+1, officeRel, rels[i], rels[i+1])
	}
	return b.String() + `</Relationships>`
}

func officeTypes(overrides ...string) string {
	var b strings.Builder
	b.WriteString(officeXML + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` + officeCTRel)
	for i := 0; i+1 < len(overrides); i += 2 {
		fmt.Fprintf(&b, `<Override PartName="%s" ContentType="application/vnd.openxmlformats-%s+xml"/>`, overrides[i], overrides[i+1])
	}
	return b.String() + `</Types>`
}

func officeBlankParts(kind string) []string {
	switch kind {
	case "docx":
		style := func(id, name, size string, bold bool) string {
			rpr := `<w:sz w:val="` + size + `"/>`
			if bold {
				rpr = `<w:b/>` + rpr
			}
			return `<w:style w:type="paragraph" w:styleId="` + id + `"><w:name w:val="` + name + `"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:keepNext/><w:spacing w:before="240" w:after="120"/></w:pPr><w:rPr>` + rpr + `</w:rPr></w:style>`
		}
		return []string{
			"[Content_Types].xml", officeTypes("/word/document.xml", "officedocument.wordprocessingml.document.main", "/word/styles.xml", "officedocument.wordprocessingml.styles"),
			"_rels/.rels", officeRels("officeDocument", "word/document.xml"),
			"word/_rels/document.xml.rels", officeRels("styles", "styles.xml"),
			"word/document.xml", officeXML + `<w:document ` + officeNSW + `><w:body><w:p/><w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr></w:body></w:document>`,
			"word/styles.xml", officeXML + `<w:styles ` + officeNSW + `><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Calibri" w:cs="Calibri"/><w:sz w:val="22"/><w:lang w:val="en-US"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>` +
				`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>` +
				style("Title", "Title", "56", false) + style("Heading1", "heading 1", "32", true) + style("Heading2", "heading 2", "26", true) + style("Heading3", "heading 3", "24", true) +
				`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:ind w:left="720"/></w:pPr></w:style></w:styles>`,
		}
	case "xlsx":
		return []string{
			"[Content_Types].xml", officeTypes("/xl/workbook.xml", "officedocument.spreadsheetml.sheet.main", "/xl/worksheets/sheet1.xml", "officedocument.spreadsheetml.worksheet", "/xl/styles.xml", "officedocument.spreadsheetml.styles"),
			"_rels/.rels", officeRels("officeDocument", "xl/workbook.xml"),
			"xl/_rels/workbook.xml.rels", officeRels("worksheet", "worksheets/sheet1.xml", "styles", "styles.xml"),
			"xl/workbook.xml", officeXML + `<workbook ` + officeNSS + `><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
			"xl/worksheets/sheet1.xml", officeXML + `<worksheet ` + officeNSS + `><sheetData/></worksheet>`,
			"xl/styles.xml", officeXML + `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/><name val="Calibri"/><family val="2"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`,
		}
	case "pptx":
		group := `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`
		shape := func(id, name, ph string, y, h int, body string) string {
			return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%s" name="%s"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr>%s</p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="1524000" y="%d"/><a:ext cx="9144000" cy="%d"/></a:xfrm></p:spPr><p:txBody><a:bodyPr%s/>%s</p:txBody></p:sp>`, id, name, ph, y, h, map[string]string{"2": ` anchor="b"`}[id], body)
		}
		title := func(body string) string {
			return shape("2", "Title 1", `<p:ph type="ctrTitle"/>`, 1122363, 2387600, body)
		}
		sub := func(body string) string {
			return shape("3", "Subtitle 2", `<p:ph type="subTitle" idx="1"/>`, 3602038, 1655762, body)
		}
		empty := `<a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p>`
		layoutTitle := `<a:lstStyle><a:lvl1pPr algn="ctr"><a:defRPr sz="6000"/></a:lvl1pPr></a:lstStyle><a:p><a:r><a:rPr lang="en-US"/><a:t>Click to add title</a:t></a:r></a:p>`
		layoutSub := `<a:lstStyle><a:lvl1pPr marL="0" indent="0" algn="ctr"><a:buNone/><a:defRPr sz="2400"/></a:lvl1pPr></a:lstStyle><a:p><a:r><a:rPr lang="en-US"/><a:t>Click to add subtitle</a:t></a:r></a:p>`
		clr := func(name, v string) string { return `<a:` + name + `><a:srgbClr val="` + v + `"/></a:` + name + `>` }
		fill := `<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`
		line := `<a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>`
		theme := officeXML + `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme"><a:themeElements><a:clrScheme name="Office">` +
			`<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
			clr("dk2", "44546A") + clr("lt2", "E7E6E6") + clr("accent1", "4472C4") + clr("accent2", "ED7D31") + clr("accent3", "A5A5A5") + clr("accent4", "FFC000") + clr("accent5", "5B9BD5") + clr("accent6", "70AD47") + clr("hlink", "0563C1") + clr("folHlink", "954F72") +
			`</a:clrScheme><a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme>` +
			`<a:fmtScheme name="Office"><a:fillStyleLst>` + fill + fill + fill + `</a:fillStyleLst><a:lnStyleLst>` + line + line + line + `</a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst>` + fill + fill + fill + `</a:bgFillStyleLst></a:fmtScheme></a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`
		return []string{
			"[Content_Types].xml", officeTypes("/ppt/presentation.xml", "officedocument.presentationml.presentation.main", "/ppt/slideMasters/slideMaster1.xml", "officedocument.presentationml.slideMaster", "/ppt/slideLayouts/slideLayout1.xml", "officedocument.presentationml.slideLayout", "/ppt/slides/slide1.xml", "officedocument.presentationml.slide", "/ppt/theme/theme1.xml", "officedocument.theme"),
			"_rels/.rels", officeRels("officeDocument", "ppt/presentation.xml"),
			"ppt/_rels/presentation.xml.rels", officeRels("slideMaster", "slideMasters/slideMaster1.xml", "slide", "slides/slide1.xml", "theme", "theme/theme1.xml"),
			"ppt/presentation.xml", officeXML + `<p:presentation ` + officeNSP + `><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:sldIdLst><p:sldId id="256" r:id="rId2"/></p:sldIdLst><p:sldSz cx="12192000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`,
			"ppt/slideMasters/_rels/slideMaster1.xml.rels", officeRels("slideLayout", "../slideLayouts/slideLayout1.xml", "theme", "../theme/theme1.xml"),
			"ppt/slideMasters/slideMaster1.xml", officeXML + `<p:sldMaster ` + officeNSP + `><p:cSld><p:spTree>` + group + title(layoutTitle) + sub(layoutSub) + `</p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>`,
			"ppt/slideLayouts/_rels/slideLayout1.xml.rels", officeRels("slideMaster", "../slideMasters/slideMaster1.xml"),
			"ppt/slideLayouts/slideLayout1.xml", officeXML + `<p:sldLayout ` + officeNSP + ` type="title" preserve="1"><p:cSld name="Title Slide"><p:spTree>` + group + title(layoutTitle) + sub(layoutSub) + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`,
			"ppt/slides/_rels/slide1.xml.rels", officeRels("slideLayout", "../slideLayouts/slideLayout1.xml"),
			"ppt/slides/slide1.xml", officeXML + `<p:sld ` + officeNSP + `><p:cSld><p:spTree>` + group + title(empty) + sub(empty) + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`,
			"ppt/theme/theme1.xml", theme,
		}
	}
	return nil
}

func OfficeBlank(kind string) []byte {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	parts := officeBlankParts(kind)
	for i := 0; i+1 < len(parts); i += 2 {
		w, _ := z.Create(parts[i])
		io.WriteString(w, parts[i+1])
	}
	z.Close()
	return buf.Bytes()
}

func (a *Archive) CreateDocument(ctx context.Context, access, name string) (*File, error) {
	ext := strings.ToLower(path.Ext(name))
	if strings.TrimSpace(strings.TrimSuffix(name, path.Ext(name))) == "" {
		return nil, fmt.Errorf("archive.create: the file name before %s must not be empty: %w", ext, httpErrBadRequest)
	}
	var b []byte
	switch ext {
	case ".docx", ".xlsx", ".pptx":
		b = OfficeBlank(ext[1:])
	default:
		return nil, fmt.Errorf("archive.create: %q is not a document; end the name in .docx, .xlsx or .pptx: %w", name, httpErrBadRequest)
	}
	return a.AddFile(ctx, access, name, nil, bytes.NewReader(b))
}

// OfficeView is OfficeRead with what it takes to draw the file as it looks: formatting, numbering, number formats,
// positions and pictures, only where the file says them. It is the canvas's: archive.read stays what an agent edits by.
func OfficeView(b []byte, name string) (*OfficeModel, error) {
	m, err := OfficeRead(b, name)
	if err != nil {
		return nil, err
	}
	p, err := OfficeOpen(b)
	if err != nil {
		return nil, err
	}
	switch m.Type {
	case "docx":
		w, err := officeOpenWord(p)
		if err != nil {
			return nil, err
		}
		w.view(m)
	case "xlsx":
		bk, err := officeOpenBook(p)
		if err != nil {
			return nil, err
		}
		bk.view(m)
	default:
		d, err := officeOpenDeck(p)
		if err != nil {
			return nil, err
		}
		if err := d.view(m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// OfficeMedia is an embedded picture of a file, by the part name OfficeView gave it.
func OfficeMedia(b []byte, part string) ([]byte, error) {
	p, err := OfficeOpen(b)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(part, "/media/") || !p.Has(part) {
		return nil, fmt.Errorf("%s: %w", part, ErrNotFound)
	}
	return p.Part(part)
}

func officeInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// officeOn reads a toggle like <w:b/>, <w:b w:val="0"/> or b="1": set, and whether it is on.
func officeOn(v string) bool { return v != "0" && v != "false" && v != "off" && v != "none" }

// officeAttr is an attribute by its local name, whatever its prefix.
func officeAttr(n *OfficeNode, local string) string {
	for _, a := range n.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// officeByLocal finds elements by their local name, for those whose prefix is declared on the element itself.
func officeByLocal(n *OfficeNode, local string) []*OfficeNode {
	var out []*OfficeNode
	for _, k := range n.Kids {
		if k.Local == local {
			out = append(out, k)
		}
		out = append(out, officeByLocal(k, local)...)
	}
	return out
}

func officeTarget(rels []OfficeRel, id string) string {
	for _, r := range rels {
		if r.ID == id {
			return r.Target
		}
	}
	return ""
}

func officeRelOf(rels []OfficeRel, kind string) string {
	for _, r := range rels {
		if strings.HasSuffix(r.Type, "/"+kind) {
			return r.Target
		}
	}
	return ""
}

// officeDrawable says whether a browser draws a picture, which EMF and WMF it does not.
func officeDrawable(part string) bool {
	switch strings.ToLower(path.Ext(part)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp":
		return true
	}
	return false
}

// officeShade applies to a "#RRGGBB" the changes a color's element can carry: lumMod and lumOff in HSL, as fractions,
// and a mix toward white (t > 0) or black (t < 0) for tint and shade.
func officeShade(hex string, lumMod, lumOff, t float64) string {
	if len(hex) != 7 {
		return hex
	}
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return hex
	}
	r, g, b := float64(v>>16&255)/255, float64(v>>8&255)/255, float64(v&255)/255
	if lumMod != 1 || lumOff != 0 {
		hi, lo := max(r, g, b), min(r, g, b)
		l := (hi + lo) / 2
		var h, s float64
		if hi != lo {
			d := hi - lo
			if s = d / (hi + lo); l > 0.5 {
				s = d / (2 - hi - lo)
			}
			switch hi {
			case r:
				h = (g - b) / d
				if g < b {
					h += 6
				}
			case g:
				h = (b-r)/d + 2
			default:
				h = (r-g)/d + 4
			}
			h /= 6
		}
		l = min(1, max(0, l*lumMod+lumOff))
		q := l + s - l*s
		if l < 0.5 {
			q = l * (1 + s)
		}
		p := 2*l - q
		ch := func(t float64) float64 {
			t -= math.Floor(t)
			switch {
			case t < 1.0/6:
				return p + (q-p)*6*t
			case t < 0.5:
				return q
			case t < 2.0/3:
				return p + (q-p)*(2.0/3-t)*6
			}
			return p
		}
		r, g, b = ch(h+1.0/3), ch(h), ch(h-1.0/3)
	}
	mix := func(c float64) int {
		if t > 0 {
			c += (1 - c) * t
		} else {
			c *= 1 + t
		}
		return int(math.Round(c * 255))
	}
	return fmt.Sprintf("#%02X%02X%02X", mix(r), mix(g), mix(b))
}

// A docx: runs, numbering, tables' merged cells, pictures, the header and the footer.

var officeHeadingStyle = regexp.MustCompile(`(?i)^(title|subtitle|heading)`)

type officeWordStyle struct {
	based, numID, ilvl, jc string
	rpr                    *OfficeNode
}

func (x *officeWord) view(m *OfficeModel) {
	rels, _ := x.pkg.Rels(officeDocPart)
	styles := map[string]officeWordStyle{}
	if b, err := x.pkg.Part(officeRelOf(rels, "styles")); err == nil {
		if doc, err := OfficeParse(b); err == nil {
			for _, st := range OfficeRoot(doc).Children(x.w, "style") {
				var v officeWordStyle
				if n := st.Child(x.w, "basedOn"); n != nil {
					v.based = n.Attrv(x.w + ":val")
				}
				v.rpr = st.Child(x.w, "rPr")
				if ppr := st.Child(x.w, "pPr"); ppr != nil {
					if n := ppr.Child(x.w, "jc"); n != nil {
						v.jc = n.Attrv(x.w + ":val")
					}
					if np := ppr.Child(x.w, "numPr"); np != nil {
						if n := np.Child(x.w, "numId"); n != nil {
							v.numID = n.Attrv(x.w + ":val")
						}
						if n := np.Child(x.w, "ilvl"); n != nil {
							v.ilvl = n.Attrv(x.w + ":val")
						}
					}
				}
				styles[st.Attrv(x.w+":styleId")] = v
			}
		}
	}
	// A style and those it is based on, nearest first.
	chain := func(id string) []officeWordStyle {
		var out []officeWordStyle
		for i := 0; id != "" && i < 16; i++ {
			st, ok := styles[id]
			if !ok {
				break
			}
			out = append(out, st)
			id = st.based
		}
		return out
	}
	levels := x.numbering(rels)
	counts := map[string][]int{}
	for i, p := range x.ps {
		mp := &m.Paragraphs[i]
		ppr := p.Child(x.w, "pPr")
		var own []officeWordStyle
		numID, ilvl, jc := "", "", ""
		if ppr != nil {
			if st := ppr.Child(x.w, "pStyle"); st != nil {
				own = chain(st.Attrv(x.w + ":val"))
			}
			if n := ppr.Child(x.w, "jc"); n != nil {
				jc = n.Attrv(x.w + ":val")
			}
			if np := ppr.Child(x.w, "numPr"); np != nil {
				if n := np.Child(x.w, "numId"); n != nil {
					numID = n.Attrv(x.w + ":val")
				}
				if n := np.Child(x.w, "ilvl"); n != nil {
					ilvl = n.Attrv(x.w + ":val")
				}
			}
		}
		styled := numID == ""
		for _, st := range own {
			jc = cmp.Or(jc, st.jc)
			numID = cmp.Or(numID, st.numID)
			ilvl = cmp.Or(ilvl, st.ilvl)
		}
		switch jc {
		case "center", "right", "both":
			mp.Align = jc
		case "end":
			mp.Align = "right"
		}
		if numID != "" && numID != "0" {
			mp.Level, _ = strconv.Atoi(ilvl)
			mp.Marker = officeMarker(levels[numID], counts, numID, mp.Level)
			// A style's own list, like List Bullet 2's, is deeper by its indent rather than by its level.
			if styled && mp.Level == 0 {
				mp.Level = max(0, int(levels[numID][0].left/360)-1)
			}
		}
		// A heading's look is the canvas's; the rest of a paragraph style's formatting reaches its text.
		var base []*OfficeNode
		if !officeHeadingStyle.MatchString(mp.Style) {
			for _, st := range own {
				if st.rpr != nil {
					base = append(base, st.rpr)
				}
			}
		}
		mp.Spans = x.spans(p, rels, base, chain)
		for _, d := range p.Find(x.w, "drawing") {
			for _, blip := range officeByLocal(d, "blip") {
				part := officeTarget(rels, officeAttr(blip, "embed"))
				if part == "" || !officeDrawable(part) {
					continue
				}
				im := OfficeImage{Part: part}
				if e := officeByLocal(d, "extent"); len(e) > 0 {
					im.Box.W, im.Box.H = officeInt(e[0].Attrv("cx")), officeInt(e[0].Attrv("cy"))
				}
				mp.Images = append(mp.Images, im)
			}
		}
	}
	for i, t := range x.tbls {
		m.Tables[i].Grid = x.grid(t)
	}
	if sects := x.doc.Find(x.w, "sectPr"); len(sects) > 0 {
		sect := sects[len(sects)-1]
		if sz := sect.Child(x.w, "pgSz"); sz != nil {
			m.Width, m.Height = officeInt(sz.Attrv(x.w+":w")), officeInt(sz.Attrv(x.w+":h"))
		}
		m.Header = x.margin(sect, "headerReference", rels)
		m.Footer = x.margin(sect, "footerReference", rels)
	}
}

// margin is the text of a section's usual header or footer.
func (x *officeWord) margin(sect *OfficeNode, ref string, rels []OfficeRel) string {
	for _, r := range sect.Children(x.w, ref) {
		if r.Attrv(x.w+":type") != "default" && r.Attrv(x.w+":type") != "" {
			continue
		}
		b, err := x.pkg.Part(officeTarget(rels, officeAttr(r, "id")))
		if err != nil {
			continue
		}
		doc, err := OfficeParse(b)
		if err != nil {
			continue
		}
		var lines []string
		for _, p := range doc.Find(x.w, "p") {
			if t := strings.TrimSpace(x.tn.text(p)); t != "" {
				lines = append(lines, t)
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

type officeLevel struct {
	format, text string
	start        int
	left         int64
}

// numbering is each list's levels, from numbering.xml.
func (x *officeWord) numbering(rels []OfficeRel) map[string]map[int]officeLevel {
	out := map[string]map[int]officeLevel{}
	b, err := x.pkg.Part(officeRelOf(rels, "numbering"))
	if err != nil {
		return out
	}
	doc, err := OfficeParse(b)
	if err != nil {
		return out
	}
	val := func(n *OfficeNode, local string) string {
		if k := n.Child(x.w, local); k != nil {
			return k.Attrv(x.w + ":val")
		}
		return ""
	}
	read := func(lvl *OfficeNode, into map[int]officeLevel) {
		i, _ := strconv.Atoi(lvl.Attrv(x.w + ":ilvl"))
		start, err := strconv.Atoi(val(lvl, "start"))
		if err != nil {
			start = 1
		}
		var left int64
		if ppr := lvl.Child(x.w, "pPr"); ppr != nil {
			if n := ppr.Child(x.w, "ind"); n != nil {
				left = officeInt(n.Attrv(x.w + ":left"))
			}
		}
		into[i] = officeLevel{val(lvl, "numFmt"), val(lvl, "lvlText"), start, left}
	}
	abstract := map[string]map[int]officeLevel{}
	root := OfficeRoot(doc)
	for _, a := range root.Children(x.w, "abstractNum") {
		ls := map[int]officeLevel{}
		for _, lvl := range a.Children(x.w, "lvl") {
			read(lvl, ls)
		}
		abstract[a.Attrv(x.w+":abstractNumId")] = ls
	}
	for _, n := range root.Children(x.w, "num") {
		ls := map[int]officeLevel{}
		maps.Copy(ls, abstract[val(n, "abstractNumId")])
		for _, o := range n.Children(x.w, "lvlOverride") {
			if lvl := o.Child(x.w, "lvl"); lvl != nil {
				read(lvl, ls)
			}
		}
		out[n.Attrv(x.w+":numId")] = ls
	}
	return out
}

func officeRoman(n int) string {
	var b strings.Builder
	for _, r := range []struct {
		v int
		s string
	}{{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"}, {100, "c"}, {90, "xc"}, {50, "l"}, {40, "xl"}, {10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"}} {
		for ; n >= r.v; n -= r.v {
			b.WriteString(r.s)
		}
	}
	return b.String()
}

func officeOrdinal(n int, format string) string {
	switch format {
	case "lowerLetter", "upperLetter":
		s := strings.Repeat(string(rune('a'+(n-1)%26)), (n-1)/26+1)
		if format == "upperLetter" {
			return strings.ToUpper(s)
		}
		return s
	case "lowerRoman":
		return officeRoman(n)
	case "upperRoman":
		return strings.ToUpper(officeRoman(n))
	case "decimalZero":
		return fmt.Sprintf("%02d", n)
	}
	return strconv.Itoa(n)
}

// officeMarker counts a list item and says what it shows, from its level's text like "%1." or a bullet's character.
func officeMarker(levels map[int]officeLevel, counts map[string][]int, id string, level int) string {
	l, ok := levels[level]
	if !ok {
		return ""
	}
	c := counts[id]
	for len(c) <= level {
		c = append(c, 0)
	}
	if c[level] == 0 {
		c[level] = l.start
	} else {
		c[level]++
	}
	c = c[:level+1]
	counts[id] = c
	switch l.format {
	case "none":
		return ""
	case "bullet":
		switch {
		case l.text == "o":
			return "◦"
		case l.text == "":
			return ""
		case []rune(l.text)[0] == 0xF0A7:
			return "▪"
		case []rune(l.text)[0] >= 0xE000 && []rune(l.text)[0] <= 0xF8FF:
			return "•"
		}
		return l.text
	}
	text := l.text
	for i := 0; i < len(c); i++ {
		n := c[i]
		f := l.format
		if o, ok := levels[i]; ok && i != level {
			f = o.format
		}
		text = strings.ReplaceAll(text, "%"+strconv.Itoa(i+1), officeOrdinal(n, f))
	}
	return text
}

// spans is a paragraph's text by formatting: its runs', and through base the paragraph style's. Nil if it has none.
func (x *officeWord) spans(p *OfficeNode, rels []OfficeRel, base []*OfficeNode, chain func(string) []officeWordStyle) []OfficeSpan {
	on := func(rprs []*OfficeNode, local string) bool {
		for _, r := range rprs {
			if n := r.Child(x.w, local); n != nil {
				return officeOn(n.Attrv(x.w + ":val"))
			}
		}
		return false
	}
	var out []OfficeSpan
	formatted := false
	var last *OfficeNode
	for _, seg := range x.tn.segments(p) {
		if seg.run == last && len(out) > 0 {
			out[len(out)-1].Text += seg.text
			continue
		}
		last = seg.run
		var rprs []*OfficeNode
		if r := seg.run.Child(x.w, "rPr"); r != nil {
			rprs = append(rprs, r)
			if n := r.Child(x.w, "rStyle"); n != nil {
				for _, st := range chain(n.Attrv(x.w + ":val")) {
					if st.rpr != nil {
						rprs = append(rprs, st.rpr)
					}
				}
			}
		}
		rprs = append(rprs, base...)
		sp := OfficeSpan{Text: seg.text, Bold: on(rprs, "b"), Italic: on(rprs, "i"), Under: on(rprs, "u"), Strike: on(rprs, "strike"), Link: seg.run.Parent != nil && seg.run.Parent.Is(x.w, "hyperlink")}
		for _, r := range rprs {
			if n := r.Child(x.w, "color"); n != nil {
				if c := n.Attrv(x.w + ":val"); c != "auto" && len(c) == 6 {
					sp.Color = "#" + strings.ToUpper(c)
				}
				break
			}
		}
		if sp.Bold || sp.Italic || sp.Under || sp.Strike || sp.Link || sp.Color != "" {
			formatted = true
		}
		out = append(out, sp)
	}
	if !formatted {
		return nil
	}
	return out
}

// grid is a table's cells as they sit on its columns: how far each spans, and which a merge above covers.
func (x *officeWord) grid(t *OfficeNode) [][]OfficeGridCell {
	var out [][]OfficeGridCell
	open := map[int][2]int{} // a column's merge still going: the row and cell it started at
	for r, tr := range x.rows(t) {
		var row []OfficeGridCell
		col := 0
		for _, tc := range x.cells(tr) {
			g := OfficeGridCell{Span: 1, Rows: 1}
			if pr := tc.Child(x.w, "tcPr"); pr != nil {
				if n := pr.Child(x.w, "gridSpan"); n != nil {
					g.Span = max(1, int(officeInt(n.Attrv(x.w+":val"))))
				}
				if n := pr.Child(x.w, "shd"); n != nil {
					if c := n.Attrv(x.w + ":fill"); c != "auto" && len(c) == 6 {
						g.Fill = "#" + strings.ToUpper(c)
					}
				}
				if n := pr.Child(x.w, "vMerge"); n != nil {
					if v := n.Attrv(x.w + ":val"); v == "restart" {
						open[col] = [2]int{r, len(row)}
					} else if at, ok := open[col]; ok {
						g.Skip = true
						out[at[0]][at[1]].Rows++
					}
				} else {
					delete(open, col)
				}
			}
			row = append(row, g)
			col += g.Span
		}
		out = append(out, row)
	}
	return out
}

// An xlsx: number formats, each format of a cell, and column widths.

var officeBuiltinFormats = map[int]string{1: "0", 2: "0.00", 3: "#,##0", 4: "#,##0.00", 9: "0%", 10: "0.00%", 11: "0.00E+00", 12: "# ?/?", 13: "# ??/??", 14: "m/d/yyyy", 15: "d-mmm-yy", 16: "d-mmm", 17: "mmm-yy", 18: "h:mm AM/PM", 19: "h:mm:ss AM/PM", 20: "h:mm", 21: "h:mm:ss", 22: "m/d/yyyy h:mm", 37: "#,##0;(#,##0)", 38: "#,##0;[Red](#,##0)", 39: "#,##0.00;(#,##0.00)", 40: "#,##0.00;[Red](#,##0.00)", 45: "mm:ss", 46: "[h]:mm:ss", 47: "mm:ss.0", 48: "##0.0E+0", 49: "@"}

var officeIndexed = strings.Fields("000000 FFFFFF FF0000 00FF00 0000FF FFFF00 FF00FF 00FFFF 000000 FFFFFF FF0000 00FF00 0000FF FFFF00 FF00FF 00FFFF 800000 008000 000080 808000 800080 008080 C0C0C0 808080 9999FF 993366 FFFFCC CCFFFF 660066 FF8080 0066CC CCCCFF 000080 FF00FF FFFF00 00FFFF 800080 800000 008080 0000FF 00CCFF CCFFFF CCFFCC FFFF99 99CCFF FF99CC CC99FF FFCC99 3366FF 33CCCC 99CC00 FFCC00 FF9900 FF6600 666699 969696 003366 339966 003300 333300 993300 993366 333399 333333")

// bookColor is a color element's value: its rgb, its place in the theme or the old palette, with its tint.
func officeBookColor(n *OfficeNode, theme []string) string {
	if n == nil {
		return ""
	}
	hex := ""
	if v := n.Attrv("rgb"); len(v) == 8 {
		hex = "#" + strings.ToUpper(v[2:])
	} else if v, ok := n.Get("theme"); ok {
		if i := int(officeInt(v)); i < len(theme) {
			hex = theme[i]
		}
	} else if v, ok := n.Get("indexed"); ok {
		if i := int(officeInt(v)); i < len(officeIndexed) {
			hex = "#" + officeIndexed[i]
		}
	}
	if t, err := strconv.ParseFloat(n.Attrv("tint"), 64); err == nil && hex != "" {
		return officeShade(hex, 1, 0, t)
	}
	return hex
}

func (bk *officeBook) view(m *OfficeModel) {
	var theme []string
	rels, _ := bk.pkg.Rels(bk.wbPart)
	if b, err := bk.pkg.Part(officeRelOf(rels, "theme")); err == nil {
		s := officeReadTheme(b)
		for _, k := range []string{"lt1", "dk1", "lt2", "dk2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hlink", "folHlink"} {
			theme = append(theme, s[k])
		}
	}
	formats := []string{}
	if b, err := bk.pkg.Part(officeRelOf(rels, "styles")); err == nil {
		if doc, err := OfficeParse(b); err == nil {
			m.Styles, formats = officeBookStyles(doc, theme)
		}
	}
	for i := range m.Sheets {
		s, err := bk.sheet(m.Sheets[i].Name)
		if err != nil {
			continue
		}
		sheet := &m.Sheets[i]
		if cols := OfficeRoot(s.doc).Child(s.x, "cols"); cols != nil {
			for _, c := range cols.Children(s.x, "col") {
				w, _ := strconv.ParseFloat(c.Attrv("width"), 64)
				sheet.Cols = append(sheet.Cols, OfficeCol{int(officeInt(c.Attrv("min"))), int(officeInt(c.Attrv("max"))), w, officeOn(cmp.Or(c.Attrv("hidden"), "0"))})
			}
		}
		masters := s.sharedMasters()
		cells := []OfficeCell{}
		for _, r := range s.rows() {
			for _, c := range s.cells(r) {
				cell := OfficeCell{Ref: OfficeColName(c.r) + strconv.Itoa(r.r), Value: s.value(c.n), Formula: s.formula(c.n, masters), Style: int(officeInt(c.n.Attrv("s")))}
				if cell.Style >= len(m.Styles) {
					cell.Style = 0
				}
				if t := c.n.Attrv("t"); cell.Value != "" && (t == "" || t == "n") && cell.Style < len(formats) {
					cell.Text, cell.Ink = officeNumber(cell.Value, formats[cell.Style])
					if cell.Text == cell.Value {
						cell.Text = ""
					}
				}
				if cell.Value != "" || cell.Formula != "" || cell.Style != 0 {
					cells = append(cells, cell)
				}
			}
		}
		sheet.Cells = cells
	}
}

// officeBookStyles is each cell format of a workbook, and its number format's code by the same index.
func officeBookStyles(doc *OfficeNode, theme []string) ([]OfficeStyle, []string) {
	root := OfficeRoot(doc)
	x, _ := OfficePrefixOf(doc, officeXNS...)
	list := func(name, item string) []*OfficeNode {
		if n := root.Child(x, name); n != nil {
			return n.Children(x, item)
		}
		return nil
	}
	custom := map[int]string{}
	for _, f := range list("numFmts", "numFmt") {
		custom[int(officeInt(f.Attrv("numFmtId")))] = f.Attrv("formatCode")
	}
	fonts, fills, borders := list("fonts", "font"), list("fills", "fill"), list("borders", "border")
	var styles []OfficeStyle
	var formats []string
	for _, xf := range list("cellXfs", "xf") {
		var st OfficeStyle
		if i := int(officeInt(xf.Attrv("fontId"))); i < len(fonts) {
			f := fonts[i]
			toggle := func(local string) bool {
				n := f.Child(x, local)
				return n != nil && officeOn(n.Attrv("val"))
			}
			st.Bold, st.Italic, st.Under, st.Strike = toggle("b"), toggle("i"), toggle("u"), toggle("strike")
			st.Color = officeBookColor(f.Child(x, "color"), theme)
			if n := f.Child(x, "sz"); n != nil {
				st.Size, _ = strconv.ParseFloat(n.Attrv("val"), 64)
			}
		}
		if i := int(officeInt(xf.Attrv("fillId"))); i < len(fills) {
			if p := fills[i].Child(x, "patternFill"); p != nil && p.Attrv("patternType") == "solid" {
				st.Fill = officeBookColor(p.Child(x, "fgColor"), theme)
			}
		}
		if i := int(officeInt(xf.Attrv("borderId"))); i < len(borders) {
			side := func(local string) string {
				n := borders[i].Child(x, local)
				if n == nil || n.Attrv("style") == "" {
					return ""
				}
				return cmp.Or(officeBookColor(n.Child(x, "color"), theme), "auto")
			}
			st.Top, st.Right, st.Bottom, st.Left = side("top"), side("right"), side("bottom"), side("left")
		}
		if al := xf.Child(x, "alignment"); al != nil {
			st.Align, st.VAlign = al.Attrv("horizontal"), al.Attrv("vertical")
			st.Wrap = officeOn(cmp.Or(al.Attrv("wrapText"), "0"))
			st.Indent = int(officeInt(al.Attrv("indent")))
		}
		id := int(officeInt(xf.Attrv("numFmtId")))
		styles = append(styles, st)
		formats = append(formats, cmp.Or(custom[id], officeBuiltinFormats[id]))
	}
	return styles, formats
}

var (
	officeFormatColor = regexp.MustCompile(`(?i)^\[(red|blue|green|black|white|yellow|magenta|cyan)\]`)
	officeFormatTag   = regexp.MustCompile(`\[[^\]]*\]`)
	officeFormatDate  = regexp.MustCompile(`(?i)[ymdhs]`)
	officeFormatNum   = regexp.MustCompile(`[0#?][0#?,.]*(?:E[+-]0+)?|\.[0#?]+`)
	officeInkNames    = map[string]string{"red": "#FF0000", "blue": "#0000FF", "green": "#00FF00", "black": "#000000", "white": "#FFFFFF", "yellow": "#FFFF00", "magenta": "#FF00FF", "cyan": "#00FFFF"}
)

// officeNumber shows a cell's number as format says, and the color the format gives it. What it does not know how to
// show, like a fraction, it leaves as the file holds it.
func officeNumber(value, format string) (string, string) {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value, ""
	}
	if format == "" || strings.EqualFold(format, "General") || format == "@" || strings.Contains(format, "?/?") {
		if format == "" || strings.EqualFold(format, "General") {
			return officeGeneral(n), ""
		}
		return value, ""
	}
	// Up to four sections: positive, negative, zero, text. A second section's number is shown without its sign.
	var sections []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(format); i++ {
		c := format[i]
		switch {
		case c == '"':
			quoted = !quoted
		case c == '\\' && i+1 < len(format):
			cur.WriteByte(c)
			i++
			c = format[i]
		case c == ';' && !quoted:
			sections = append(sections, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	sections = append(sections, cur.String())
	section, abs := sections[0], false
	switch {
	case n < 0 && len(sections) > 1:
		section, abs = sections[1], true
	case n == 0 && len(sections) > 2:
		section = sections[2]
	}
	if abs {
		n = -n
	}
	ink := ""
	if m := officeFormatColor.FindStringSubmatch(section); m != nil {
		ink = officeInkNames[strings.ToLower(m[1])]
		section = section[len(m[0]):]
	}
	// Literals out of the way, as control characters that stand for their index.
	var lits []string
	var code strings.Builder
	lit := func(s string) {
		lits = append(lits, s)
		code.WriteByte(1)
	}
	for i := 0; i < len(section); i++ {
		c := section[i]
		switch c {
		case '"':
			j := strings.IndexByte(section[i+1:], '"')
			if j < 0 {
				j = len(section) - i - 1
			}
			lit(section[i+1 : i+1+j])
			i += j + 1
		case '\\':
			if i+1 < len(section) {
				i++
				lit(section[i : i+1])
			}
		case '_':
			i++
			lit(" ")
		case '*':
			i++
		case '[':
			j := strings.IndexByte(section[i:], ']')
			if j < 0 {
				code.WriteByte(c)
				continue
			}
			tag := section[i : i+j+1]
			if strings.HasPrefix(tag, "[$") {
				sym, _, _ := strings.Cut(tag[2:len(tag)-1], "-")
				lit(sym)
			} else {
				code.WriteString(tag)
			}
			i += j
		default:
			code.WriteByte(c)
		}
	}
	text := code.String()
	bare := officeFormatTag.ReplaceAllString(text, "")
	out := ""
	if officeFormatDate.MatchString(bare) && !strings.Contains(bare, "#") {
		out = officeDate(n, text)
	} else {
		out = officeFormatNum.ReplaceAllStringFunc(text, func(p string) string { return officeDigits(n, p, strings.Contains(text, "%")) })
		if !officeFormatNum.MatchString(text) {
			out = text
		}
		if n < 0 {
			out = "-" + strings.TrimPrefix(out, "-")
		}
	}
	var b strings.Builder
	li := 0
	for i := 0; i < len(out); i++ {
		if out[i] == 1 {
			if li < len(lits) {
				b.WriteString(lits[li])
				li++
			}
			continue
		}
		b.WriteByte(out[i])
	}
	return b.String(), ink
}

func officeGeneral(n float64) string {
	if n == math.Trunc(n) && math.Abs(n) < 1e11 {
		return strconv.FormatInt(int64(n), 10)
	}
	s := strconv.FormatFloat(n, 'g', 10, 64)
	if !strings.Contains(s, "e") {
		return s
	}
	return strconv.FormatFloat(n, 'E', 2, 64)
}

// officeDigits is a number by one run of a format's digit placeholders, like #,##0.00 or 0.00E+00.
func officeDigits(n float64, p string, percent bool) string {
	if percent {
		n *= 100
	}
	if i := strings.IndexAny(p, "Ee"); i >= 0 {
		dec := 0
		if d := strings.IndexByte(p[:i], '.'); d >= 0 {
			dec = i - d - 1
		}
		s := strconv.FormatFloat(n, 'E', dec, 64)
		m, e, _ := strings.Cut(s, "E")
		return m + "E" + e[:1] + fmt.Sprintf("%02s", strings.TrimLeft(e[1:], "0"))
	}
	whole, frac, _ := strings.Cut(p, ".")
	comma := strings.Contains(whole, ",")
	for strings.HasSuffix(whole, ",") {
		n /= 1000
		whole = whole[:len(whole)-1]
	}
	need, opt := strings.Count(frac, "0"), strings.Count(frac, "#")+strings.Count(frac, "?")
	s := strconv.FormatFloat(math.Abs(n), 'f', need+opt, 64)
	in, fr, _ := strings.Cut(s, ".")
	for len(fr) > need && strings.HasSuffix(fr, "0") {
		fr = fr[:len(fr)-1]
	}
	if z := strings.Count(whole, "0"); len(in) < z {
		in = strings.Repeat("0", z-len(in)) + in
	} else if z == 0 && in == "0" {
		in = ""
	}
	if comma {
		for i := len(in) - 3; i > 0; i -= 3 {
			in = in[:i] + "," + in[i:]
		}
	}
	if fr != "" {
		return in + "." + fr
	}
	return in
}

// officeDate is a serial date by a format's y, m, d, h and s, m being minutes after an h or before an s.
func officeDate(n float64, code string) string {
	t := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(n * float64(24*time.Hour)).Round(time.Second))
	lower := strings.ToLower(code)
	h12 := strings.Contains(lower, "am/pm") || strings.Contains(lower, "a/p")
	var b strings.Builder
	lastH := false
	for i := 0; i < len(code); {
		if strings.HasPrefix(lower[i:], "am/pm") {
			b.WriteString(t.Format("PM"))
			i += 5
			continue
		}
		c := lower[i]
		if !strings.ContainsRune("ymdhs", rune(c)) {
			if c == '[' {
				j := strings.IndexByte(code[i:], ']')
				if j > 0 {
					tag := lower[i+1 : i+j]
					if strings.Trim(tag, "hms") == "" {
						switch tag[0] {
						case 'h':
							fmt.Fprintf(&b, "%d", int(n*24))
							lastH = true
						case 'm':
							fmt.Fprintf(&b, "%d", int(n*24*60))
						case 's':
							fmt.Fprintf(&b, "%d", int(n*24*3600))
						}
					}
					i += j + 1
					continue
				}
			}
			b.WriteByte(code[i])
			i++
			continue
		}
		k := i
		for k < len(code) && lower[k] == c {
			k++
		}
		run := k - i
		rest := strings.TrimLeft(lower[k:], " :\x01")
		switch c {
		case 'y':
			if run > 2 {
				b.WriteString(t.Format("2006"))
			} else {
				b.WriteString(t.Format("06"))
			}
		case 'm':
			if run <= 2 && (lastH || strings.HasPrefix(rest, "s")) {
				fmt.Fprintf(&b, "%0*d", run, t.Minute())
				lastH = false
			} else {
				switch run {
				case 1:
					b.WriteString(t.Format("1"))
				case 2:
					b.WriteString(t.Format("01"))
				case 3:
					b.WriteString(t.Format("Jan"))
				case 5:
					b.WriteString(t.Format("Jan")[:1])
				default:
					b.WriteString(t.Format("January"))
				}
			}
		case 'd':
			switch run {
			case 1:
				b.WriteString(t.Format("2"))
			case 2:
				b.WriteString(t.Format("02"))
			case 3:
				b.WriteString(t.Format("Mon"))
			default:
				b.WriteString(t.Format("Monday"))
			}
		case 'h':
			h := t.Hour()
			if h12 {
				h = (h+11)%12 + 1
			}
			fmt.Fprintf(&b, "%0*d", run, h)
			lastH = true
		case 's':
			fmt.Fprintf(&b, "%0*d", run, t.Second())
			lastH = false
		}
		i = k
	}
	return b.String()
}

// A pptx: where everything is and how it looks, each thing found on the slide, then its layout, then its master.

type officeLayer struct {
	part string
	root *OfficeNode
	a, p string
	rels []OfficeRel
}

func (l *officeLayer) tree() *OfficeNode {
	if c := l.root.Child(l.p, "cSld"); c != nil {
		return c.Child(l.p, "spTree")
	}
	return nil
}

func (d *officeDeck) layer(part string) (*officeLayer, error) {
	b, err := d.pkg.Part(part)
	if err != nil {
		return nil, err
	}
	doc, err := OfficeParse(b)
	if err != nil {
		return nil, err
	}
	a, _ := OfficePrefixOf(doc, officeANS...)
	p, _ := OfficePrefixOf(doc, officePNS...)
	rels, _ := d.pkg.Rels(part)
	return &officeLayer{part, OfficeRoot(doc), a, p, rels}, nil
}

// officeReadTheme is a theme's colors by their names, like dk1 and accent1.
func officeReadTheme(b []byte) map[string]string {
	out := map[string]string{}
	doc, err := OfficeParse(b)
	if err != nil {
		return out
	}
	a, _ := OfficePrefixOf(doc, officeANS...)
	for _, s := range OfficeRoot(doc).Find(a, "clrScheme") {
		for _, k := range s.Kids {
			if len(k.Kids) == 0 {
				continue
			}
			v := cmp.Or(k.Kids[0].Attrv("lastClr"), k.Kids[0].Attrv("val"))
			if len(v) == 6 {
				out[k.Local] = "#" + strings.ToUpper(v)
			}
		}
		break
	}
	return out
}

// officePaint reads colors and fills in a deck's theme.
type officePaint struct {
	theme, clrMap map[string]string
}

// color is the color element under n, with its changes. "" if it has none, or is the placeholder color of a style.
func (c officePaint) color(n *OfficeNode) string {
	if n == nil {
		return ""
	}
	for _, k := range n.Kids {
		hex := ""
		switch k.Local {
		case "srgbClr":
			hex = "#" + strings.ToUpper(k.Attrv("val"))
		case "sysClr":
			hex = "#" + strings.ToUpper(k.Attrv("lastClr"))
		case "schemeClr":
			v := k.Attrv("val")
			hex = c.theme[cmp.Or(c.clrMap[v], v)]
		default:
			continue
		}
		mod, off, t := 1.0, 0.0, 0.0
		for _, e := range k.Kids {
			v := float64(officeInt(e.Attrv("val"))) / 100000
			switch e.Local {
			case "lumMod":
				mod = v
			case "lumOff":
				off = v
			case "tint":
				t = 1 - v
			case "shade":
				t = v - 1
			}
		}
		return officeShade(hex, mod, off, t)
	}
	return ""
}

// fill is a shape's or background's fill: a solid color or a gradient's first, and whether it says none.
func (c officePaint) fill(pr *OfficeNode) (string, bool) {
	if pr == nil {
		return "", false
	}
	for _, k := range pr.Kids {
		switch k.Local {
		case "noFill":
			return "", true
		case "solidFill":
			return c.color(k), false
		case "gradFill":
			if gs := officeByLocal(k, "gs"); len(gs) > 0 {
				return c.color(gs[0]), false
			}
		}
	}
	return "", false
}

func (d *officeDeck) view(m *OfficeModel) error {
	m.Width, m.Height = d.w, d.h
	for i := range d.slides {
		sl, err := d.layer(d.slides[i])
		if err != nil {
			return err
		}
		lay, err := d.layer(officeRelOf(sl.rels, "slideLayout"))
		if err != nil {
			return err
		}
		mas, err := d.layer(officeRelOf(lay.rels, "slideMaster"))
		if err != nil {
			return err
		}
		paint := officePaint{theme: map[string]string{}, clrMap: map[string]string{}}
		if b, err := d.pkg.Part(officeRelOf(mas.rels, "theme")); err == nil {
			paint.theme = officeReadTheme(b)
		}
		if cm := mas.root.Child(mas.p, "clrMap"); cm != nil {
			for _, a := range cm.Attr {
				paint.clrMap[a.Name.Local] = a.Value
			}
		}
		s := &m.Slides[i]
		v := officeSlideView{d: d, paint: paint, sl: sl, lay: lay, mas: mas}
		v.background(s)
		z := 0
		for _, l := range []*officeLayer{mas, lay} {
			if l == mas && lay.root.Attrv("showMasterSp") == "0" {
				continue
			}
			v.decor(l, s, &z)
		}
		// The slide's own, in document order: shapes with text keep the index they are edited by.
		n := 0
		tree := sl.tree()
		if tree == nil {
			continue
		}
		var walk func(*OfficeNode)
		walk = func(g *OfficeNode) {
			for _, k := range g.Kids {
				switch {
				case k.Is(sl.p, "grpSp"):
					walk(k)
				case k.Is(sl.p, "sp"):
					z++
					if k.Child(sl.p, "txBody") != nil {
						if n < len(s.Shapes) {
							v.shape(sl, k, &s.Shapes[n])
							s.Shapes[n].Z = z
						}
						n++
					} else {
						var sh OfficeShape
						v.shape(sl, k, &sh)
						sh.Z = z
						if sh.Box != nil && (sh.Fill != "" || sh.Line != "") {
							s.Decor = append(s.Decor, sh)
						}
					}
				case k.Is(sl.p, "pic"):
					z++
					if im, ok := v.picture(sl, k); ok {
						im.Z = z
						s.Pictures = append(s.Pictures, im)
					}
				}
			}
		}
		walk(tree)
	}
	return nil
}

type officeSlideView struct {
	d            *officeDeck
	paint        officePaint
	sl, lay, mas *officeLayer
}

func (v officeSlideView) background(s *OfficeSlide) {
	for _, l := range []*officeLayer{v.sl, v.lay, v.mas} {
		c := l.root.Child(l.p, "cSld")
		if c == nil {
			continue
		}
		bg := c.Child(l.p, "bg")
		if bg == nil {
			continue
		}
		if pr := bg.Child(l.p, "bgPr"); pr != nil {
			if blips := officeByLocal(pr, "blip"); len(blips) > 0 {
				if part := officeTarget(l.rels, officeAttr(blips[0], "embed")); officeDrawable(part) {
					s.Backdrop = part
				}
			}
			s.Background, _ = v.paint.fill(pr)
		} else if ref := bg.Child(l.p, "bgRef"); ref != nil {
			s.Background = v.paint.color(ref)
		}
		return
	}
}

func (v officeSlideView) decor(l *officeLayer, s *OfficeSlide, z *int) {
	tree := l.tree()
	if tree == nil {
		return
	}
	for _, k := range tree.Kids {
		switch {
		case k.Is(l.p, "sp"):
			if officeHolder(k, l.p) != nil {
				continue
			}
			var sh OfficeShape
			v.shape(l, k, &sh)
			if sh.Box == nil {
				continue
			}
			*z++
			sh.Z = *z
			for _, sp := range k.Find(l.a, "p") {
				sh.Paragraphs = append(sh.Paragraphs, OfficeParagraph{Text: (officeTextNames{l.a, "p", "r", "t", false}).text(sp)})
			}
			v.paragraphs(l, k, &sh)
			s.Decor = append(s.Decor, sh)
		case k.Is(l.p, "pic"):
			if im, ok := v.picture(l, k); ok {
				*z++
				im.Z = *z
				s.Pictures = append(s.Pictures, im)
			}
		}
	}
}

// officeHolder is a shape's placeholder element, if it is one.
func officeHolder(sp *OfficeNode, p string) *OfficeNode {
	for _, nv := range sp.Kids {
		if strings.HasPrefix(nv.Local, "nv") {
			if nvp := nv.Child(p, "nvPr"); nvp != nil {
				return nvp.Child(p, "ph")
			}
		}
	}
	return nil
}

// inherited is the shape a placeholder takes after in a layer: by its idx, or else by its kind.
func (v officeSlideView) inherited(l *officeLayer, ph *OfficeNode) *OfficeNode {
	tree := l.tree()
	if tree == nil || ph == nil {
		return nil
	}
	kind := func(t string) string {
		switch t {
		case "ctrTitle":
			return "title"
		case "", "subTitle", "obj", "tbl", "chart", "dgm", "media", "clipArt", "pic":
			return "body"
		}
		return t
	}
	var byKind *OfficeNode
	for _, k := range tree.Find(l.p, "sp") {
		h := officeHolder(k, l.p)
		if h == nil {
			continue
		}
		if idx := ph.Attrv("idx"); idx != "" && h.Attrv("idx") == idx {
			return k
		}
		if byKind == nil && kind(h.Attrv("type")) == kind(ph.Attrv("type")) {
			byKind = k
		}
	}
	return byKind
}

// lineage is a shape and those it takes after, each with its layer, nearest first.
func (v officeSlideView) lineage(l *officeLayer, sp *OfficeNode) []officeOwned {
	out := []officeOwned{{l, sp}}
	ph := officeHolder(sp, l.p)
	if ph == nil {
		return out
	}
	if l == v.sl {
		if k := v.inherited(v.lay, ph); k != nil {
			out = append(out, officeOwned{v.lay, k})
		}
	}
	if l != v.mas {
		if k := v.inherited(v.mas, ph); k != nil {
			out = append(out, officeOwned{v.mas, k})
		}
	}
	return out
}

type officeOwned struct {
	l  *officeLayer
	sp *OfficeNode
}

// box is where a shape is, in slide coordinates through the groups it is in.
func (v officeSlideView) box(chain []officeOwned, pr string) *OfficeBox {
	for _, o := range chain {
		spPr := o.sp.Child(o.l.p, pr)
		if spPr == nil {
			continue
		}
		xf := spPr.Child(o.l.a, "xfrm")
		if xf == nil {
			continue
		}
		off, ext := xf.Child(o.l.a, "off"), xf.Child(o.l.a, "ext")
		if off == nil || ext == nil {
			continue
		}
		b := &OfficeBox{X: officeInt(off.Attrv("x")), Y: officeInt(off.Attrv("y")), W: officeInt(ext.Attrv("cx")), H: officeInt(ext.Attrv("cy")), Rot: float64(officeInt(xf.Attrv("rot"))) / 60000}
		for g := o.sp.Nearest(o.l.p, "grpSp"); g != nil; g = g.Nearest(o.l.p, "grpSp") {
			pr := g.Child(o.l.p, "grpSpPr")
			if pr == nil {
				break
			}
			gx := pr.Child(o.l.a, "xfrm")
			if gx == nil {
				break
			}
			go1, ge, gc, gce := gx.Child(o.l.a, "off"), gx.Child(o.l.a, "ext"), gx.Child(o.l.a, "chOff"), gx.Child(o.l.a, "chExt")
			if go1 == nil || ge == nil || gc == nil || gce == nil || officeInt(gce.Attrv("cx")) == 0 || officeInt(gce.Attrv("cy")) == 0 {
				break
			}
			sx, sy := float64(officeInt(ge.Attrv("cx")))/float64(officeInt(gce.Attrv("cx"))), float64(officeInt(ge.Attrv("cy")))/float64(officeInt(gce.Attrv("cy")))
			b.X = officeInt(go1.Attrv("x")) + int64(float64(b.X-officeInt(gc.Attrv("x")))*sx)
			b.Y = officeInt(go1.Attrv("y")) + int64(float64(b.Y-officeInt(gc.Attrv("y")))*sy)
			b.W, b.H = int64(float64(b.W)*sx), int64(float64(b.H)*sy)
		}
		return b
	}
	return nil
}

func (v officeSlideView) picture(l *officeLayer, pic *OfficeNode) (OfficeImage, bool) {
	blips := officeByLocal(pic, "blip")
	if len(blips) == 0 {
		return OfficeImage{}, false
	}
	part := officeTarget(l.rels, officeAttr(blips[0], "embed"))
	box := v.box(v.lineage(l, pic), "spPr")
	if !officeDrawable(part) || box == nil {
		return OfficeImage{}, false
	}
	return OfficeImage{Part: part, Box: *box}, true
}

// shape fills in where a shape is, how it looks, and how its text does.
func (v officeSlideView) shape(l *officeLayer, sp *OfficeNode, sh *OfficeShape) {
	chain := v.lineage(l, sp)
	sh.Box = v.box(chain, "spPr")
	if pr := sp.Child(l.p, "spPr"); pr != nil {
		fill, none := v.paint.fill(pr)
		sh.Fill = fill
		style := sp.Child(l.p, "style")
		if fill == "" && !none && style != nil {
			if r := style.Child(l.a, "fillRef"); r != nil && r.Attrv("idx") != "0" {
				sh.Fill = v.paint.color(r)
			}
		}
		if ln := pr.Child(l.a, "ln"); ln != nil {
			sh.Line, _ = v.paint.fill(ln)
		} else if style != nil {
			if r := style.Child(l.a, "lnRef"); r != nil && r.Attrv("idx") != "0" {
				sh.Line = v.paint.color(r)
			}
		}
		if g := pr.Child(l.a, "prstGeom"); g != nil {
			if g := g.Attrv("prst"); g == "ellipse" || g == "roundRect" {
				sh.Geom = g
			}
		}
	}
	for _, o := range chain {
		tx := o.sp.Child(o.l.p, "txBody")
		if tx == nil {
			continue
		}
		bp := tx.Child(o.l.a, "bodyPr")
		if bp == nil {
			continue
		}
		if sh.Anchor == "" {
			sh.Anchor = bp.Attrv("anchor")
		}
		if sh.Inset == nil && bp.Attrv("lIns")+bp.Attrv("tIns")+bp.Attrv("rIns")+bp.Attrv("bIns") != "" {
			ins := func(a string, def int64) int64 {
				if s, ok := bp.Get(a); ok {
					return officeInt(s)
				}
				return def
			}
			sh.Inset = []int64{ins("lIns", 91440), ins("tIns", 45720), ins("rIns", 91440), ins("bIns", 45720)}
		}
	}
	if sh.Inset == nil {
		sh.Inset = []int64{91440, 45720, 91440, 45720}
	}
	v.paragraphs(l, sp, sh)
}

// paragraphs fills in a shape's text: alignment, bullets and each run's look, through its list styles and those of what
// it takes after, then the master's text style, then what the shape's style says.
func (v officeSlideView) paragraphs(l *officeLayer, sp *OfficeNode, sh *OfficeShape) {
	tx := sp.Child(l.p, "txBody")
	if tx == nil {
		return
	}
	chain := v.lineage(l, sp)
	ph := officeHolder(sp, l.p)
	master := "otherStyle"
	if ph != nil {
		master = "bodyStyle"
		if t := ph.Attrv("type"); t == "title" || t == "ctrTitle" {
			master = "titleStyle"
		}
	}
	levels := func(n int) []officeOwned {
		name := "lvl" + strconv.Itoa(n+1) + "pPr"
		var out []officeOwned
		for _, o := range chain {
			if t := o.sp.Child(o.l.p, "txBody"); t != nil {
				if s := t.Child(o.l.a, "lstStyle"); s != nil {
					if k := s.Child(o.l.a, name); k != nil {
						out = append(out, officeOwned{o.l, k})
					}
				}
			}
		}
		if ts := v.mas.root.Child(v.mas.p, "txStyles"); ts != nil {
			if s := ts.Child(v.mas.p, master); s != nil {
				if k := s.Child(v.mas.a, name); k != nil {
					out = append(out, officeOwned{v.mas, k})
				}
			}
		}
		return out
	}
	scale := 1.0
	if bp := tx.Child(l.a, "bodyPr"); bp != nil {
		if fit := bp.Child(l.a, "normAutofit"); fit != nil {
			if f := officeInt(fit.Attrv("fontScale")); f > 0 {
				scale = float64(f) / 100000
			}
		}
	}
	ink := ""
	if style := sp.Child(l.p, "style"); style != nil {
		ink = v.paint.color(style.Child(l.a, "fontRef"))
	}
	tn := officeTextNames{l.a, "p", "r", "t", false}
	autos := map[int]int{}
	for i, p := range tx.Children(l.a, "p") {
		if i >= len(sh.Paragraphs) {
			break
		}
		mp := &sh.Paragraphs[i]
		pPr := p.Child(l.a, "pPr")
		lvl := 0
		if pPr != nil {
			lvl = int(officeInt(pPr.Attrv("lvl")))
		}
		ls := levels(lvl)
		algn := ""
		if pPr != nil {
			algn = pPr.Attrv("algn")
		}
		for _, o := range ls {
			algn = cmp.Or(algn, o.sp.Attrv("algn"))
		}
		switch algn {
		case "ctr":
			mp.Align = "center"
		case "r":
			mp.Align = "right"
		case "just":
			mp.Align = "both"
		}
		mp.Level = lvl
		mp.Marker = officeBullet(pPr, ls, l.a, autos, lvl)
		var spans []OfficeSpan
		var last *OfficeNode
		for _, seg := range tn.segments(p) {
			if seg.run == last && len(spans) > 0 {
				spans[len(spans)-1].Text += seg.text
				continue
			}
			last = seg.run
			var rPr *OfficeNode
			if seg.run != nil {
				rPr = seg.run.Child(l.a, "rPr")
			}
			spans = append(spans, v.run(rPr, ls, l.a, ink, scale, seg.text))
		}
		if len(spans) == 0 {
			var end *OfficeNode
			if e := p.Child(l.a, "endParaRPr"); e != nil {
				end = e
			}
			spans = []OfficeSpan{v.run(end, ls, l.a, ink, scale, "")}
		}
		mp.Spans = spans
	}
}

// officeBullet is what a paragraph of a shape starts with, the first of its own and its list styles' to say.
func officeBullet(pPr *OfficeNode, ls []officeOwned, a string, autos map[int]int, lvl int) string {
	nodes := []*OfficeNode{}
	if pPr != nil {
		nodes = append(nodes, pPr)
	}
	for _, o := range ls {
		nodes = append(nodes, o.sp)
	}
	for _, n := range nodes {
		for _, k := range n.Kids {
			switch k.Local {
			case "buNone":
				return ""
			case "buChar":
				c := k.Attrv("char")
				if c == "" {
					return ""
				}
				if r := []rune(c)[0]; r >= 0xE000 && r <= 0xF8FF || c == "§" || c == "Ø" {
					return "•"
				}
				return c
			case "buAutoNum":
				autos[lvl]++
				for k := range autos {
					if k > lvl {
						delete(autos, k)
					}
				}
				t := k.Attrv("type")
				f := "decimal"
				switch {
				case strings.HasPrefix(t, "alphaLc"):
					f = "lowerLetter"
				case strings.HasPrefix(t, "alphaUc"):
					f = "upperLetter"
				case strings.HasPrefix(t, "romanLc"):
					f = "lowerRoman"
				case strings.HasPrefix(t, "romanUc"):
					f = "upperRoman"
				}
				s := officeOrdinal(autos[lvl]+max(int(officeInt(k.Attrv("startAt"))), 1)-1, f)
				switch {
				case strings.HasSuffix(t, "ParenBoth"):
					return "(" + s + ")"
				case strings.HasSuffix(t, "ParenR"):
					return s + ")"
				}
				return s + "."
			}
		}
	}
	return ""
}

// run is a run's look, each thing from the first of its own properties and its list styles' to say it.
func (v officeSlideView) run(rPr *OfficeNode, ls []officeOwned, a, ink string, scale float64, text string) OfficeSpan {
	var props []*OfficeNode
	if rPr != nil {
		props = append(props, rPr)
	}
	for _, o := range ls {
		if d := o.sp.Child(o.l.a, "defRPr"); d != nil {
			props = append(props, d)
		}
	}
	sp := OfficeSpan{Text: text, Size: 18 * scale, Color: ink}
	on := func(name string, set *bool) {
		for _, r := range props {
			if s, ok := r.Get(name); ok {
				*set = officeOn(s)
				return
			}
		}
	}
	on("b", &sp.Bold)
	on("i", &sp.Italic)
	on("u", &sp.Under)
	if s := func() string {
		for _, r := range props {
			if s, ok := r.Get("strike"); ok {
				return s
			}
		}
		return ""
	}(); s != "" && s != "noStrike" {
		sp.Strike = true
	}
	for _, r := range props {
		if s, ok := r.Get("sz"); ok {
			sp.Size = float64(officeInt(s)) / 100 * scale
			break
		}
	}
	for _, r := range props {
		if f, none := v.paint.fill(r); f != "" || none {
			sp.Color = f
			break
		}
	}
	if sp.Color == "" {
		sp.Color = v.paint.theme[cmp.Or(v.paint.clrMap["tx1"], "dk1")]
	}
	return sp
}
