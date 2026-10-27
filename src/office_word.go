package main

import (
	"fmt"
	"strconv"
	"strings"
)

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
