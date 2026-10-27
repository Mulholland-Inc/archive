package main

import (
	"fmt"
	"strings"
)

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
