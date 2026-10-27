package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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
