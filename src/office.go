package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
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
