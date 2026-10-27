package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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
