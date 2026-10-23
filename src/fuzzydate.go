package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// fuzzy_date is a date as people know it: "3 Mar 1890", but also "about 1890", "before 1900",
// "between 1890 and 1895", "Mar 1890" or "the 1890s". It is stored as EDTF (ISO 8601-2, level 1)
// with the earliest and latest day it can mean, so it sorts and filters like a date:
//
//	{"edtf": "1890~", "min": "1890-01-01", "max": "1890-12-31", "text": null}
//	{"edtf": "../1900", "min": null, "max": "1900-12-31", "text": null}
//
// Text no date can be read from, such as "in the spring after the flood", is kept as text with
// the other three null, so importing an old family tree never stops at a date. "About" does not widen
// min and max: "about 1890" filters as 1890, and the ~ says it is approximate.

var (
	fuzzyPoint   = regexp.MustCompile(`^(\d{4}|\d{3}X|\d{2}XX)(?:-(\d{2}))?(?:-(\d{2}))?([~?%])?$`)
	fuzzyQual    = regexp.MustCompile(`^(ABT|ABOUT|CA|CIRCA|C|EST|CAL|BEF|BEFORE|AFT|AFTER)\.?\s+(.+)$`)
	fuzzyBetween = regexp.MustCompile(`^(?:BET|BETWEEN)\.?\s+(.+?)\s+(?:AND|&)\s+(.+)$`)
	fuzzyFromTo  = regexp.MustCompile(`^FROM\s+(.+?)\s+TO\s+(.+)$`)
	fuzzyOneEnd  = regexp.MustCompile(`^(FROM|TO)\s+(.+)$`)
	fuzzySpan    = regexp.MustCompile(`^(\d{3,4})\s*[-–]\s*(\d{3,4})$`)
	fuzzyDecade  = regexp.MustCompile(`^(\d{3})0'?S$`)
	fuzzyMonY    = regexp.MustCompile(`^([a-z]+)\.?,? (\d{3,4})$`)
	fuzzyGlued   = regexp.MustCompile(`([a-z])(\d)|(\d)([a-z])`)
	fuzzyClock   = regexp.MustCompile(`\s+\d{1,2}:\d{2}(:\d{2})?\s*([ap]\.?m\.?)?$`)
)

// fuzzyParse reads a fuzzy_date from what a person, a model or a GEDCOM file wrote.
func fuzzyParse(in any, h Hints) ([]Candidate, error) {
	if m, ok := in.(map[string]any); ok {
		if e, _ := m["edtf"].(string); e != "" {
			v, err := fuzzyValue(e)
			if err != nil {
				return nil, err
			}
			return []Candidate{{Value: v, Confidence: 1}}, nil
		}
		if t, _ := m["text"].(string); strings.TrimSpace(t) != "" {
			return []Candidate{{Value: fuzzyText(t), Confidence: 1}}, nil
		}
		return nil, errors.New("fuzzy date must have edtf or text")
	}
	s, ok := in.(string)
	if !ok {
		if n, ok := semNumStr(in); ok {
			s = n
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty date")
	}
	if v, err := fuzzyValue(s); err == nil {
		return []Candidate{{Value: v, Confidence: 1}}, nil
	}
	e, conf := fuzzyEDTF(s, h)
	if e == "" {
		return []Candidate{{Value: fuzzyText(s), Confidence: 0.3}}, nil
	}
	v, err := fuzzyValue(e)
	if err != nil {
		return []Candidate{{Value: fuzzyText(s), Confidence: 0.3}}, nil
	}
	return []Candidate{{Value: v, Confidence: conf}}, nil
}

// fuzzyEDTF turns GEDCOM's and people's phrasing into EDTF, with how sure the reading is.
func fuzzyEDTF(s string, h Hints) (string, float64) {
	u := strings.ToUpper(strings.Join(strings.Fields(s), " "))
	both := func(a, b, mid string) (string, float64) {
		x, cx := fuzzyAt(a, h)
		y, cy := fuzzyAt(b, h)
		if x == "" || y == "" {
			return "", 0
		}
		return x + mid + y, cx * cy
	}
	if m := fuzzyQual.FindStringSubmatch(u); m != nil {
		p, c := fuzzyAt(m[2], h)
		switch {
		case p == "":
			return "", 0
		case m[1] == "BEF" || m[1] == "BEFORE":
			return "../" + p, c
		case m[1] == "AFT" || m[1] == "AFTER":
			return p + "/..", c
		case m[1] == "EST" || m[1] == "CAL":
			return p + "?", c
		}
		return p + "~", c
	}
	if m := fuzzyBetween.FindStringSubmatch(u); m != nil {
		return both(m[1], m[2], "/")
	}
	if m := fuzzyFromTo.FindStringSubmatch(u); m != nil {
		return both(m[1], m[2], "/")
	}
	if m := fuzzySpan.FindStringSubmatch(u); m != nil {
		return both(m[1], m[2], "/")
	}
	if m := fuzzyOneEnd.FindStringSubmatch(u); m != nil {
		p, c := fuzzyAt(m[2], h)
		if p == "" {
			return "", 0
		}
		if m[1] == "FROM" {
			return p + "/..", c
		}
		return "../" + p, c
	}
	if m := fuzzyDecade.FindStringSubmatch(strings.TrimPrefix(u, "THE ")); m != nil {
		return m[1] + "X", 1
	}
	return fuzzyAt(u, h)
}

// fuzzyAt reads one point: a year, a month and year, or a whole date in any order the date type
// reads, with the confidence of its first reading (3/4/1890 is not sure whether it is March or
// April).
func fuzzyAt(s string, h Hints) (string, float64) {
	t := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, ",", " ")))
	t = fuzzyClock.ReplaceAllString(t, "") // Ancestry's "11/18/2014 7:36:37 PM": the day is enough
	t = strings.Join(strings.Fields(fuzzyGlued.ReplaceAllString(t, "$1$3 $2$4")), " ")
	if q := strings.TrimSuffix(t, "?"); q != t {
		if p, c := fuzzyAt(q, h); p != "" && !strings.ContainsAny(p, "~?%") {
			return p + "?", c
		}
		return "", 0
	}
	if n, err := strconv.Atoi(t); err == nil && len(t) >= 3 && len(t) <= 4 {
		return fmt.Sprintf("%04d", n), 1
	}
	if m := fuzzyMonY.FindStringSubmatch(t); m != nil {
		if mo := semMonths[m[1]]; mo > 0 {
			y, _ := strconv.Atoi(m[2])
			return fmt.Sprintf("%04d-%02d", y, mo), 1
		}
	}
	if c := semDateCands(t, h); len(c) > 0 {
		return c[0].Value.(string), c[0].Confidence
	}
	return "", 0
}

// fuzzyValue checks an EDTF string and works out the earliest and latest day it can mean.
func fuzzyValue(e string) (map[string]any, error) {
	start, end, interval := strings.Cut(e, "/")
	if !interval {
		lo, hi, err := fuzzyBounds(e)
		if err != nil {
			return nil, err
		}
		return map[string]any{"edtf": e, "min": lo, "max": hi, "text": nil}, nil
	}
	if (start == ".." || start == "") && (end == ".." || end == "") {
		return nil, errors.New("an interval needs a start or an end")
	}
	var lo, hi any
	if start != ".." && start != "" {
		l, _, err := fuzzyBounds(start)
		if err != nil {
			return nil, err
		}
		lo = l
	}
	if end != ".." && end != "" {
		_, h, err := fuzzyBounds(end)
		if err != nil {
			return nil, err
		}
		hi = h
	}
	if lo != nil && hi != nil && lo.(string) > hi.(string) {
		return nil, errors.New("end is before start")
	}
	return map[string]any{"edtf": e, "min": lo, "max": hi, "text": nil}, nil
}

func fuzzyText(s string) map[string]any {
	return map[string]any{"edtf": nil, "min": nil, "max": nil, "text": strings.TrimSpace(s)}
}

// fuzzyBounds gives the first and last day of one EDTF point: 1890 is the whole year, 1890-03
// the whole month, 189X the whole decade.
func fuzzyBounds(p string) (string, string, error) {
	m := fuzzyPoint.FindStringSubmatch(p)
	if m == nil {
		return "", "", fmt.Errorf("%q is not an EDTF date such as 1890, 1890-03, 1890-03-12, 1890~, 189X or 1890/1895", p)
	}
	ylo, _ := strconv.Atoi(strings.ReplaceAll(m[1], "X", "0"))
	yhi, _ := strconv.Atoi(strings.ReplaceAll(m[1], "X", "9"))
	if strings.Contains(m[1], "X") && m[2] != "" {
		return "", "", fmt.Errorf("%q has a month on an unknown year", p)
	}
	if m[2] == "" {
		return fmt.Sprintf("%04d-01-01", ylo), fmt.Sprintf("%04d-12-31", yhi), nil
	}
	mo, _ := strconv.Atoi(m[2])
	if mo < 1 || mo > 12 {
		return "", "", fmt.Errorf("%q has no month %d", p, mo)
	}
	if m[3] == "" {
		last := time.Date(ylo, time.Month(mo)+1, 0, 0, 0, 0, 0, time.UTC)
		return fmt.Sprintf("%04d-%02d-01", ylo, mo), last.Format(time.DateOnly), nil
	}
	d, _ := strconv.Atoi(m[3])
	day, ok := semYMD(ylo, mo, d)
	if !ok {
		return "", "", fmt.Errorf("%q is not a day", p)
	}
	return day, day, nil
}

func fuzzyCheck(v any) error {
	m, _ := v.(map[string]any)
	if t, ok := m["text"].(string); ok {
		if strings.TrimSpace(t) == "" || m["edtf"] != nil || m["min"] != nil || m["max"] != nil {
			return errors.New("text must be the date as written, with edtf, min and max null")
		}
		return nil
	}
	e, _ := m["edtf"].(string)
	want, err := fuzzyValue(e)
	if err != nil {
		return err
	}
	if m["min"] != want["min"] || m["max"] != want["max"] {
		return fmt.Errorf("min and max must be %v and %v for %s", want["min"], want["max"], e)
	}
	return nil
}

func fuzzyFormat(v any, locale string) string {
	m := v.(map[string]any)
	if t, ok := m["text"].(string); ok {
		return t
	}
	e := m["edtf"].(string)
	if a, b, ok := strings.Cut(e, "/"); ok {
		switch {
		case a == ".." || a == "":
			return "before " + fuzzyWords(b, locale)
		case b == ".." || b == "":
			return "after " + fuzzyWords(a, locale)
		}
		return fuzzyWords(a, locale) + " – " + fuzzyWords(b, locale)
	}
	return fuzzyWords(e, locale)
}

func fuzzyWords(p, locale string) string {
	q := p[len(p)-1]
	if q == '~' || q == '?' || q == '%' {
		p = p[:len(p)-1]
	}
	var w string
	switch m := fuzzyPoint.FindStringSubmatch(p); {
	case m == nil:
		w = p
	case strings.HasSuffix(m[1], "XX"):
		w = strings.Replace(m[1], "XX", "00s", 1)
	case strings.HasSuffix(m[1], "X"):
		w = strings.Replace(m[1], "X", "0s", 1)
	case m[2] == "":
		w = m[1]
	case m[3] == "":
		mo, _ := strconv.Atoi(m[2])
		w = time.Month(mo).String() + " " + m[1]
	default:
		d, _ := time.Parse(time.DateOnly, p)
		w = d.Format(semDateLayout(locale))
	}
	switch q {
	case '~':
		return "about " + w
	case '?':
		return w + "?"
	case '%':
		return "about " + w + "?"
	}
	return w
}
