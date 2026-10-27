package main

import (
	"fmt"
	"strings"
)

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
