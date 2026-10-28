package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/go-crdt/crdt/structured"
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
}
type OfficeParagraph struct {
	ID    string   `json:"id,omitempty"`
	Style string   `json:"style,omitempty"`
	List  string   `json:"list,omitempty"`
	In    string   `json:"in,omitempty"`
	Text  string   `json:"text"`
	Runs  []string `json:"runs,omitempty"`
}
type OfficeTable struct {
	ID   string     `json:"id"`
	Rows [][]string `json:"rows"`
}
type OfficeSheet struct {
	Name      string         `json:"name"`
	Dimension string         `json:"dimension,omitempty"`
	Merged    []string       `json:"merged,omitempty"`
	Tables    []OfficeXTable `json:"tables,omitempty"`
	Cells     []OfficeCell   `json:"cells"`
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
}
type OfficeSlide struct {
	Number int           `json:"slide"`
	Hidden bool          `json:"hidden,omitempty"`
	Shapes []OfficeShape `json:"shapes"`
}
type OfficeShape struct {
	Index       int               `json:"shape"`
	Name        string            `json:"name,omitempty"`
	Placeholder string            `json:"placeholder,omitempty"`
	Paragraphs  []OfficeParagraph `json:"paragraphs"`
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
	p, err := OfficeEditPackage(b, name, ops)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := p.Save(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func OfficeEditPackage(b []byte, name string, ops []OfficeOp) (*OfficePackage, error) {
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
		if st != nil && !st.updated {
			for _, ss := range p.Log[officeDocPart][n:] {
				st.update(ss)
			}
		}
		if st != nil {
			st.updated = false
		}
	}
	return p, nil
}
func officeApply(p *OfficePackage, kind string, op OfficeOp, st *officeIds) error {
	types, ok := officeOpTypes[op.Op]
	if !ok {
		return fmt.Errorf("unknown op %q; ops are replace_text, insert_paragraph_after, delete_paragraph, set_cell_text (docx), set_cell, set_formula, clear_cell, append_table_row (xlsx), replace_text, replace_shape_text (pptx)", op.Op)
	}
	if !strings.Contains(types, kind) {
		return fmt.Errorf("op %s applies to %s files, not %s", op.Op, types, kind)
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
			if op.Find == "" {
				return fmt.Errorf("replace_text needs a non-empty \"find\"")
			}
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
		if op.Find == "" {
			return fmt.Errorf("replace_text needs a non-empty \"find\"")
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
	if m, ok, err := a.ReadPage(ctx, id); ok {
		return m, err
	}
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
	f, err := fileOf(a.st.db(ctx).LockFile(ctx, id))
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	if pageIs(f.Name) {
		return a.editPage(ctx, f, ops)
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
func (x *officeWord) rows(tbl *OfficeNode) []*OfficeNode {
	var out []*OfficeNode
	for _, r := range tbl.Find(x.w, "tr") {
		if r.Nearest(x.w, "tbl") == tbl {
			out = append(out, r)
		}
	}
	return out
}
func (x *officeWord) cells(tr *OfficeNode) []*OfficeNode {
	var out []*OfficeNode
	for _, c := range tr.Find(x.w, "tc") {
		if c.Nearest(x.w, "tr") == tr {
			out = append(out, c)
		}
	}
	return out
}
func (x *officeWord) model() *OfficeModel {
	m := &OfficeModel{Type: "docx"}
	pid := map[*OfficeNode]int{}
	for i, p := range x.ps {
		pid[p] = i
	}
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
				for _, p := range c.Find(x.w, "p") {
					if p.Nearest(x.w, "tc") == c {
						parts = append(parts, x.tn.text(p))
					}
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
		for i, p := range x.ps {
			if strings.Contains(x.tn.text(p), e.Find) {
				id := "new paragraph"
				for k, off := range x.ids.start {
					if off == p.Start {
						id = "p" + strconv.Itoa(k)
					}
				}
				_ = i
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
	for _, r := range p.Find(x.w, "r") {
		if r.Nearest(x.w, "p") == p {
			if n := r.Child(x.w, "rPr"); n != nil {
				var drop []OfficeSplice
				for _, k := range n.Kids {
					if k.Local == "ins" || k.Local == "del" || k.Local == "rPrChange" {
						drop = append(drop, OfficeSplice{k.Start - n.Start, k.End - n.Start, nil})
					}
				}
				b, _ := OfficeApply(x.b[n.Start:n.End], drop)
				rPr = string(b)
			}
			break
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
func (tn officeTextNames) tElem(b []byte, t *OfficeNode, text string) []byte {
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
		return []OfficeSplice{{segs[a].t.Start, segs[a].t.End, tn.tElem(b, segs[a].t, prefix+repl+suffix)}}, nil
	}
	ss := []OfficeSplice{{segs[a].t.Start, segs[a].t.End, tn.tElem(b, segs[a].t, prefix+repl)}}
	gone := map[*OfficeNode]bool{}
	for k := a + 1; k <= z; k++ {
		if k == z && suffix != "" {
			ss = append(ss, OfficeSplice{segs[z].t.Start, segs[z].t.End, tn.tElem(b, segs[z].t, suffix)})
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
			return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%s" name="%s"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr>%s</p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="1524000" y="%d"/><a:ext cx="9144000" cy="%d"/></a:xfrm></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/>%s</p:txBody></p:sp>`, id, name, ph, y, h, body)
		}
		title := func(body string) string {
			return shape("2", "Title 1", `<p:ph type="ctrTitle"/>`, 1122363, 2387600, body)
		}
		sub := func(body string) string {
			return shape("3", "Subtitle 2", `<p:ph type="subTitle" idx="1"/>`, 3602038, 1655762, body)
		}
		empty := `<a:p><a:endParaRPr lang="en-US"/></a:p>`
		layoutTitle := `<a:p><a:pPr algn="ctr"/><a:r><a:rPr lang="en-US" sz="6000"/><a:t>Click to add title</a:t></a:r></a:p>`
		layoutSub := `<a:p><a:pPr marL="0" indent="0" algn="ctr"><a:buNone/></a:pPr><a:r><a:rPr lang="en-US" sz="2400"/><a:t>Click to add subtitle</a:t></a:r></a:p>`
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

func (a *Archive) CreateDocument(ctx context.Context, space, name string) (*File, error) {
	ext := strings.ToLower(path.Ext(name))
	if strings.TrimSpace(strings.TrimSuffix(name, path.Ext(name))) == "" {
		return nil, fmt.Errorf("archive.create: the file name before %s must not be empty: %w", ext, httpErrBadRequest)
	}
	var b []byte
	switch ext {
	case pageExt:
		b = structured.NewBlocks(pageSite).Snapshot()
	case ".docx", ".xlsx", ".pptx":
		b = OfficeBlank(ext[1:])
	default:
		return nil, fmt.Errorf("archive.create: %q is not a document; end the name in .page, .docx, .xlsx or .pptx: %w", name, httpErrBadRequest)
	}
	return a.AddFile(ctx, space, name, bytes.NewReader(b))
}
