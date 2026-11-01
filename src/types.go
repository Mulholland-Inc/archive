package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"mime"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"
	"unicode"

	"github.com/google/uuid"
	"github.com/nyaruka/phonenumbers"
	"golang.org/x/net/idna"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
	"golang.org/x/text/unicode/norm"
)

var semTransforms = map[string]func(string) (string, error){
	"trim":         func(s string) (string, error) { return strings.TrimSpace(s), nil },
	"lower":        func(s string) (string, error) { return strings.ToLower(s), nil },
	"upper":        func(s string) (string, error) { return strings.ToUpper(s), nil },
	"nfc":          func(s string) (string, error) { return norm.NFC.String(s), nil },
	"strip_spaces": func(s string) (string, error) { return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), ""), nil },

	"strip_separators": func(s string) (string, error) { return semStrip(s), nil },
	"punycode":         semDomain,
	"lower_domain": func(s string) (string, error) {
		at := strings.LastIndex(s, "@")
		if at < 0 {
			return "", errors.New("not an email address")
		}
		d, err := semDomain(s[at+1:])
		return s[:at] + "@" + d, err
	},
	"mail_address": func(s string) (string, error) {
		a, err := mail.ParseAddress(s)
		if err != nil {
			return "", errors.New("not an email address")
		}
		return a.Address, nil
	},
	"timezone": semZone,
	"rrule":    semRRule,
	"country":  semCountry,
	"locale":   semLocale,
	"uuid":     semUUID,
	"ip":       semIP,
	"mime":     semMIME,
	"bic":      semBIC,
	"ein_dash": func(s string) (string, error) {
		if len(s) != 9 || !semAllDigits(s) {
			return "", errors.New("must be 9 digits")
		}
		return s[:2] + "-" + s[2:], nil
	},
	"gtin14": func(s string) (string, error) {
		if !semAllDigits(s) || (len(s) != 8 && len(s) != 12 && len(s) != 13 && len(s) != 14) {
			return "", errors.New("GTIN must be 8, 12, 13 or 14 digits")
		}
		return strings.Repeat("0", 14-len(s)) + s, nil
	},
	"isbn13": semISBN13,
}

var semParsers = map[string]func(in any, h Hints) ([]Candidate, error){
	"date": func(in any, h Hints) ([]Candidate, error) {
		s, _ := in.(string)
		return semDateCands(s, h), nil
	},
	"timestamp": semParseTimestamp,
	"time":      semParseTime,
	"duration":  semParseDuration,
	"interval":  semParseInterval,
	"money":     semParseMoney,
	"currency": func(in any, h Hints) ([]Candidate, error) {
		s, _ := in.(string)
		return semOne(semCurrency(s, h))
	},
	"percentage": semParsePercentage,
	"number": func(in any, h Hints) ([]Candidate, error) {
		s, _ := in.(string)
		var out []Candidate
		for _, c := range semNumber(s, h) {
			f, _ := strconv.ParseFloat(c.Value.(string), 64)
			out = append(out, Candidate{Value: f, Confidence: c.Confidence})
		}
		return out, nil
	},
	"decimal": func(in any, h Hints) ([]Candidate, error) {
		if s, ok := semNumStr(in); ok {
			d, _ := semDec(s)
			return []Candidate{{Value: d, Confidence: 1}}, nil
		}
		s, _ := in.(string)
		return semNumber(s, h), nil
	},
	"boolean": func(in any, _ Hints) ([]Candidate, error) {
		s, ok := in.(string)
		if !ok {
			s, _ = semNumStr(in)
		}
		b, ok := semBools[strings.ToLower(s)]
		if !ok {
			return nil, errors.New("not a boolean")
		}
		return []Candidate{{Value: b, Confidence: 1}}, nil
	},
	"phone": semParsePhone,
	"region": func(in any, h Hints) ([]Candidate, error) {
		s, _ := in.(string)
		return semOne(semRegion(s, h))
	},
	"url":      semParseURL,
	"card":     semParseCard,
	"national": semParseNational,
	"passport": semParsePassport,
	"address":  semParseAddress,
	"name":     semParseName,
	"postcode": semParsePostcode,
	"geo":      semParseGeo,
	"quantity": semParseQuantity,
	"instant": func(in any, h Hints) ([]Candidate, error) {
		var out []Candidate
		for _, c := range semInstants(in, h) {
			out = append(out, Candidate{Value: semUTC(c.Value.(time.Time)), Confidence: c.Confidence})
		}
		return out, nil
	},
}

var semChecks = map[string]func(v any) error{
	"date": semStrCheck("must be a YYYY-MM-DD date", func(s string) bool {
		_, err := time.Parse(time.DateOnly, s)
		return err == nil && len(s) == 10
	}),
	"timestamp": func(v any) error {
		m, _ := v.(map[string]any)
		if _, ok := semParseUTC(semS(m, "utc")); !ok {
			return errors.New("utc must be an RFC 3339 UTC instant")
		}
		if z := semS(m, "zone"); semMust(semZone(z)) != z {
			return errors.New("zone must be an IANA zone")
		}
		return nil
	},
	"time": func(v any) error {
		m, _ := v.(map[string]any)
		s := semS(m, "time")
		if p, err := time.Parse(time.TimeOnly, s); err != nil || p.Format(time.TimeOnly) != s {
			return errors.New("time must be HH:MM:SS")
		}
		if z := semS(m, "zone"); z != "" && semMust(semZone(z)) != z {
			return errors.New("zone must be an IANA zone")
		}
		return nil
	},
	"duration": semStrCheck("must be a canonical ISO 8601 duration", func(s string) bool { return semMust(semDuration(s)) == s }),
	"interval": semCheckInterval,
	"currency": semStrCheck("not an ISO 4217 currency", func(s string) bool {
		u, err := currency.ParseISO(s)
		return err == nil && u.String() == s
	}),
	"money": func(v any) error {
		m, _ := v.(map[string]any)
		a, c := semS(m, "amount"), semS(m, "currency")
		d, _ := semDec(a)
		if p, ok := semAmount(d, c); !ok || p != a {
			return fmt.Errorf("amount must be a decimal string with %d fraction digits", semMinor(c))
		}
		return nil
	},
	"lei":  semStrCheck("LEI check digits do not match", func(s string) bool { return semMod97(s) == 1 }),
	"isin": semStrCheck("ISIN check digit does not match", semISIN),
	"luhn": semStrCheck("Luhn check failed", semLuhn),
	"aba": semStrCheck("not a valid ABA routing number", func(s string) bool {
		p, _ := strconv.Atoi(s[:min(2, len(s))])
		return semABACheck(s) && (p <= 12 || (p >= 21 && p <= 32) || (p >= 61 && p <= 72) || p == 80)
	}),
	"ssn_ranges": semStrCheck("not a valid SSN", semSSN),
	"national": func(v any) error {
		m, _ := v.(map[string]any)
		c, sc := semS(m, "country"), semS(m, "scheme")
		n, ok := semNationals[c]
		if !ok || n.scheme != sc {
			return fmt.Errorf("no national id scheme %q for %q", sc, c)
		}
		if !n.check(semS(m, "value")) {
			return fmt.Errorf("invalid %s", sc)
		}
		return nil
	},
	"passport": func(v any) error {
		m, _ := v.(map[string]any)
		if !semPassportRe.MatchString(semS(m, "number")) {
			return errors.New("number must be 6-9 letters or digits")
		}
		return nil
	},
	"phone": semStrCheck("not a valid E.164 number", func(s string) bool {
		n, err := phonenumbers.Parse(s, "")
		return err == nil && phonenumbers.IsValidNumber(n) && phonenumbers.Format(n, phonenumbers.E164) == s
	}),
	"address": func(v any) error {
		m, _ := v.(map[string]any)
		lines, _ := m["lines"].([]any)
		if len(lines) == 0 {
			return errors.New("lines must be a non-empty list")
		}
		for _, l := range lines {
			if s, ok := l.(string); !ok || s == "" || strings.TrimSpace(s) != s {
				return errors.New("lines must be trimmed strings")
			}
		}
		c := semS(m, "country")
		if r := semS(m, "region"); r != "" && !strings.HasPrefix(r, c+"-") {
			return errors.New("region must be in the country")
		}
		if p := semS(m, "postcode"); p != "" && semMust(semPostcode(p, c)) != p {
			return fmt.Errorf("postcode is not valid for %s", c)
		}
		return nil
	},
	"name": func(v any) error {
		m, _ := v.(map[string]any)
		for _, k := range []string{"full", "given", "family", "prefixes"} {
			if s := semS(m, k); m[k] != nil && (s == "" || strings.TrimSpace(s) != s) {
				return fmt.Errorf("%s must be a trimmed string", k)
			}
		}
		return nil
	},
	"url":    semStrCheck("not a canonical URL", func(s string) bool { return semMust(semURL(s)) == s }),
	"region": semStrCheck("not an ISO 3166-2 code", func(s string) bool { return semMust(semRegion(s, Hints{})) == s }),
	"postcode": func(v any) error {
		m, _ := v.(map[string]any)
		if code := semS(m, "code"); semMust(semPostcode(code, semS(m, "country"))) != code {
			return errors.New("not a valid postcode for its country")
		}
		return nil
	},
	"geo": func(v any) error {
		m, _ := v.(map[string]any)
		lat, _ := m["lat"].(float64)
		lon, _ := m["lon"].(float64)
		if !(lat >= -90 && lat <= 90) || !(lon >= -180 && lon <= 180) {
			return errors.New("lat must be in [-90,90] and lon in [-180,180]")
		}
		return nil
	},
	"quantity": func(v any) error {
		m, _ := v.(map[string]any)
		u, o := semS(m, "unit"), semS(m, "original_unit")
		if semUnits[u].base != u || semUnits[o].base != u {
			return errors.New("value must be in the SI base unit with a UCUM original unit")
		}
		return nil
	},
	"gtin": semStrCheck("GTIN check digit does not match", func(s string) bool { return len(s) == 14 && semGS1(s[:13]) == s[13] }),
	"isbn": semStrCheck("not a valid ISBN-13", func(s string) bool {
		return len(s) == 13 && semAllDigits(s) && (strings.HasPrefix(s, "978") || strings.HasPrefix(s, "979")) && semGS1(s[:12]) == s[12]
	}),
	"card": func(v any) error {
		m, _ := v.(map[string]any)
		if len(semS(m, "token")) != 64 || len(semS(m, "last4")) != 4 || semS(m, "brand") == "" || len(m) != 3 {
			return errors.New("want a card number")
		}
		return nil
	},
	"instant": semStrCheck("must be an RFC 3339 UTC instant", func(s string) bool {
		_, ok := semParseUTC(s)
		return ok
	}),
}

var semMasks = map[string]func(v any) string{
	"card": func(v any) string {
		m := v.(map[string]any)
		return m["brand"].(string) + " **** " + m["last4"].(string)
	},
	"mask_ssn": func(v any) string { return "***-**-" + v.(string)[5:] },
	"national": func(v any) string {
		m := v.(map[string]any)
		return m["country"].(string) + " " + m["scheme"].(string) + " " + semMask(m["value"].(string), 4)
	},
	"passport": func(v any) string {
		m := v.(map[string]any)
		return m["country"].(string) + " " + semMask(m["number"].(string), 3)
	},
}

func semStrCheck(msg string, ok func(string) bool) func(any) error {
	return func(v any) error {
		if s, _ := v.(string); !ok(s) {
			return errors.New(msg)
		}
		return nil
	}
}

func semOne(v string, err error) ([]Candidate, error) {
	if err != nil {
		return nil, err
	}
	return []Candidate{{Value: v, Confidence: 1}}, nil
}

func semS(m map[string]any, k string) string { s, _ := m[k].(string); return s }

func semFirst(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := semNumStr(m[k]); ok {
			return s
		}
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func semNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func semMust(s string, err error) string {
	if err != nil {
		return ""
	}
	return s
}

func semPlain(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func semNumStr(v any) (string, bool) {
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "", false
		}
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case json.Number:
		return x.String(), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	}
	return "", false
}

func semStrip(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '.', '/', ' ':
			return -1
		}
		return r
	}, s)
}

func semClean(s string) string { return strings.ToUpper(semStrip(s)) }

func semAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func semMask(s string, keep int) string {
	if len(s) <= keep {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-keep) + s[len(s)-keep:]
}

func semTag(h Hints) (tag language.Tag, hinted bool) {
	if h.Locale != "" {
		if t, err := language.Parse(strings.ReplaceAll(h.Locale, "_", "-")); err == nil {
			return t, true
		}
	}
	if h.Country != "" {
		if t, err := language.Parse("und-" + h.Country); err == nil {
			b, _ := t.Base()
			r, _ := t.Region()
			if t, err = language.Compose(b, r); err == nil {
				return t, true
			}
		}
	}
	return language.Und, false
}

func semHintCountry(h Hints) string {
	if h.Country != "" {
		return strings.ToUpper(h.Country)
	}
	return semLocaleRegion(h.Locale)
}

func semLocaleRegion(locale string) string {
	t, err := language.Parse(strings.ReplaceAll(locale, "_", "-"))
	if err != nil || locale == "" {
		return ""
	}
	r, _ := t.Region()
	return r.String()
}

var semSepCache sync.Map

func semSeps(t language.Tag) (group, dec string) {
	if v, ok := semSepCache.Load(t); ok {
		p := v.([2]string)
		return p[0], p[1]
	}
	s := message.NewPrinter(t).Sprint(number.Decimal(1234567.5))
	var seps []string
	for _, r := range s {
		if r < '0' || r > '9' {
			seps = append(seps, string(r))
		}
	}
	group, dec = ",", "."
	if len(seps) >= 2 {
		group, dec = seps[0], seps[len(seps)-1]
	}
	semSepCache.Store(t, [2]string{group, dec})
	return group, dec
}

var semDecIn = regexp.MustCompile(`^([+-]?)([0-9]*)(?:\.([0-9]*))?$`)

func semDec(s string) (string, bool) {
	m := semDecIn.FindStringSubmatch(s)
	if m == nil || m[2]+m[3] == "" {
		return "", false
	}
	i := strings.TrimLeft(m[2], "0")
	if i == "" {
		i = "0"
	}
	out := i
	if f := strings.TrimRight(m[3], "0"); f != "" {
		out += "." + f
	}
	if m[1] == "-" && out != "0" {
		out = "-" + out
	}
	return out, true
}

func semShift(d string, n int) string {
	sign := ""
	if strings.HasPrefix(d, "-") {
		sign, d = "-", d[1:]
	}
	i, f, _ := strings.Cut(d, ".")
	digits, point := i+f, len(i)+n
	if point < 0 {
		digits, point = strings.Repeat("0", -point)+digits, 0
	}
	if point > len(digits) {
		digits += strings.Repeat("0", point-len(digits))
	}
	out, _ := semDec(sign + digits[:point] + "." + digits[point:])
	return out
}

var semNumChars = regexp.MustCompile(`^[0-9.,]+$`)

func semNumber(s string, h Hints) []Candidate {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', ' ', ' ', '\'', '’', '_':
			return -1
		case '−':
			return '-'
		}
		return r
	}, s)
	sign := ""
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		sign, s = s[:1], s[1:]
	}
	if !semNumChars.MatchString(s) {
		return nil
	}
	one := func(x string, c float64) Candidate {
		d, ok := semDec(sign + x)
		if !ok {
			return Candidate{}
		}
		return Candidate{Value: d, Confidence: c}
	}
	dots, commas := strings.Count(s, "."), strings.Count(s, ",")
	var out []Candidate
	switch {
	case dots == 0 && commas == 0:
		out = []Candidate{one(s, 1)}
	case dots > 0 && commas > 0:
		dec, grp := ".", ","
		if strings.LastIndex(s, ",") > strings.LastIndex(s, ".") {
			dec, grp = ",", "."
		}
		if strings.Count(s, dec) > 1 {
			return nil
		}
		out = []Candidate{one(strings.ReplaceAll(strings.ReplaceAll(s, grp, ""), dec, "."), 1)}
	case dots+commas > 1:
		out = []Candidate{one(strings.NewReplacer(".", "", ",", "").Replace(s), 1)}
	default:
		sep := "."
		if commas == 1 {
			sep = ","
		}
		before, after, _ := strings.Cut(s, sep)
		asDec := one(before+"."+after, 1)
		if len(after) != 3 || before == "" || before == "0" || len(before) > 3 {
			out = []Candidate{asDec}
			break
		}
		tag, hinted := semTag(h)
		_, ldec := semSeps(tag)
		hi, lo := 0.6, 0.4
		if hinted {
			hi, lo = 0.8, 0.2
		}
		asGrp := one(before+after, lo)
		asDec.Confidence = hi
		if ldec != sep {
			asGrp.Confidence, asDec.Confidence = hi, lo
		}
		out = []Candidate{asDec, asGrp}
	}
	return slices.DeleteFunc(out, func(c Candidate) bool { return c.Value == nil })
}

func semRound12(f float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(f, 'g', 12, 64), 64)
	return r
}

func semParsePercentage(in any, h Hints) ([]Candidate, error) {
	s, ok := in.(string)
	if !ok {
		return nil, errors.New("must be a number")
	}
	pct := strings.HasSuffix(s, "%")
	var out []Candidate
	for _, c := range semNumber(strings.TrimSpace(strings.TrimSuffix(s, "%")), h) {
		d := c.Value.(string)
		f, _ := strconv.ParseFloat(d, 64)
		p, _ := strconv.ParseFloat(semShift(d, -2), 64)
		switch {
		case pct:
			out = append(out, Candidate{Value: p, Confidence: c.Confidence})
		case math.Abs(f) > 1:
			out = append(out, Candidate{Value: p, Confidence: c.Confidence * 0.9})
		default:
			out = append(out, Candidate{Value: f, Confidence: c.Confidence * 0.7}, Candidate{Value: p, Confidence: c.Confidence * 0.3})
		}
	}
	return out, nil
}

var semBools = map[string]bool{"true": true, "yes": true, "y": true, "1": true, "on": true, "ja": true, "j": true, "waar": true,
	"false": false, "no": false, "n": false, "0": false, "off": false, "nee": false, "onwaar": false}

var (
	semISODate  = regexp.MustCompile(`^([0-9]{4})[-/.]([0-9]{1,2})[-/.]([0-9]{1,2})$`)
	semNumDate  = regexp.MustCompile(`^([0-9]{1,2})([-/.])([0-9]{1,2})([-/.])([0-9]{2}|[0-9]{4})$`)
	semDMonY    = regexp.MustCompile(`^([0-9]{1,2})(?:st|nd|rd|th)?\.?\s+([\p{L}.]+),?\s+([0-9]{4})$`)
	semMonDY    = regexp.MustCompile(`^([\p{L}.]+)\s+([0-9]{1,2})(?:st|nd|rd|th)?,?\s+([0-9]{4})$`)
	semClockRe  = regexp.MustCompile(`(?i)^([0-9]{1,2})(?:[:.]([0-9]{2}))?(?::([0-9]{2})(\.[0-9]+)?)?\s*([ap])?\.?(m\.?)?$`)
	semDateTime = regexp.MustCompile(`(?i)^(.+?)(?:T|\s+|,\s*)([0-9]{1,2}[:.][0-9]{2}(?::[0-9]{2}(?:\.[0-9]+)?)?(?:\s*[ap]\.?m\.?)?)$`)
	semZoneRe   = regexp.MustCompile(`^[A-Z][A-Za-z0-9_+\-]*(/[A-Z0-9][A-Za-z0-9_+\-]*)*$`)
)

var semMonths = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6, "july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
	"januari": 1, "februari": 2, "maart": 3, "mei": 5, "juni": 6, "juli": 7, "augustus": 8, "oktober": 10,
	"jan": 1, "feb": 2, "mar": 3, "mrt": 3, "apr": 4, "jun": 6, "jul": 7, "aug": 8, "sep": 9, "sept": 9, "oct": 10, "okt": 10, "nov": 11, "dec": 12,
}

func semYMD(y, m, d int) (string, bool) {
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != m || t.Day() != d {
		return "", false
	}
	return t.Format(time.DateOnly), true
}

func semDateCands(s string, h Hints) []Candidate {
	atoi := func(x string) int { n, _ := strconv.Atoi(x); return n }
	var out []Candidate
	add := func(y, m, d int, c float64) {
		if v, ok := semYMD(y, m, d); ok {
			out = append(out, Candidate{Value: v, Confidence: c})
		}
	}
	if m := semISODate.FindStringSubmatch(s); m != nil {
		add(atoi(m[1]), atoi(m[2]), atoi(m[3]), 1)
	} else if len(s) == 8 && semAllDigits(s) {
		add(atoi(s[:4]), atoi(s[4:6]), atoi(s[6:]), 0.9)
	} else if m := semNumDate.FindStringSubmatch(s); m != nil && m[2] == m[4] {
		a, b, y := atoi(m[1]), atoi(m[3]), atoi(m[5])
		if len(m[5]) == 2 {
			y += 1900
			if y < 1950 {
				y += 100
			}
		}
		_, hinted := semTag(h)
		dmy, mdy := 0.6, 0.4
		if hinted {
			dmy, mdy = 0.8, 0.2
		}
		if semHintCountry(h) == "US" {
			dmy, mdy = mdy, dmy
		}
		if a == b {
			dmy = 1
		}
		add(y, b, a, dmy)
		add(y, a, b, mdy)
		if len(out) == 1 {
			out[0].Confidence = 1
		}
	} else if m := semDMonY.FindStringSubmatch(s); m != nil {
		if mo := semMonths[strings.TrimSuffix(strings.ToLower(m[2]), ".")]; mo > 0 {
			add(atoi(m[3]), mo, atoi(m[1]), 1)
		}
	} else if m := semMonDY.FindStringSubmatch(s); m != nil {
		if mo := semMonths[strings.TrimSuffix(strings.ToLower(m[1]), ".")]; mo > 0 {
			add(atoi(m[3]), mo, atoi(m[2]), 1)
		}
	} else if len(s) > 10 && (s[10] == 'T' || s[10] == ' ') && semISODate.MatchString(s[:10]) {
		add(atoi(s[:4]), atoi(s[5:7]), atoi(s[8:10]), 0.9)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Confidence > out[j].Confidence })
	return out
}

func semClock(s string) (h, m, sec, nsec int, ok bool) {
	g := semClockRe.FindStringSubmatch(strings.TrimSpace(s))
	if g == nil || (g[2] == "" && g[5] == "") || (g[5] == "" && g[6] != "") {
		return 0, 0, 0, 0, false
	}
	h, _ = strconv.Atoi(g[1])
	m, _ = strconv.Atoi(g[2])
	sec, _ = strconv.Atoi(g[3])
	if g[4] != "" {
		f, _ := strconv.ParseFloat("0"+g[4], 64)
		nsec = int(math.Round(f * 1e9))
	}
	if ap := strings.ToLower(g[5]); ap != "" {
		if h < 1 || h > 12 {
			return 0, 0, 0, 0, false
		}
		if h == 12 {
			h = 0
		}
		if ap == "p" {
			h += 12
		}
	}
	return h, m, sec, nsec, h < 24 && m < 60 && sec < 60
}

func semZone(s string) (string, error) {
	switch strings.ToUpper(s) {
	case "UTC", "Z", "ETC/UTC":
		return "UTC", nil
	}
	if !semZoneRe.MatchString(s) || s == "Local" {
		return "", errors.New("not an IANA zone name")
	}
	if _, err := time.LoadLocation(s); err != nil {
		return "", errors.New("unknown IANA zone")
	}
	return s, nil
}

func semLoc(zone string) *time.Location {
	if l, err := time.LoadLocation(zone); err == nil && zone != "" {
		return l
	}
	return time.UTC
}

func semUTC(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func semParseUTC(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	return t, err == nil && strings.HasSuffix(s, "Z") && semUTC(t) == s
}

func semInstants(in any, h Hints) []Candidate {
	loc := semLoc(h.Zone)
	zoneConf := 0.8
	if h.Zone != "" {
		zoneConf = 1
	}
	if n, ok := in.(float64); ok {
		if math.Abs(n) > 1e11 {
			return []Candidate{{Value: time.UnixMilli(int64(n)), Confidence: 0.8}}
		}
		return []Candidate{{Value: time.Unix(int64(n), int64((n-math.Trunc(n))*1e9)), Confidence: 0.9}}
	}
	s, _ := in.(string)
	for _, l := range []string{time.RFC3339Nano, "2006-01-02 15:04:05Z07:00", "2006-01-02T15:04Z07:00", "2006-01-02 15:04Z07:00", "2006-01-02 15:04:05 -0700"} {
		if t, err := time.Parse(l, s); err == nil {
			return []Candidate{{Value: t, Confidence: 1}}
		}
	}
	if t, err := mail.ParseDate(s); err == nil {
		return []Candidate{{Value: t, Confidence: 1}}
	}
	var out []Candidate
	if m := semDateTime.FindStringSubmatch(s); m != nil {
		if hh, mm, ss, ns, ok := semClock(m[2]); ok {
			for _, c := range semDateCands(m[1], h) {
				d, _ := time.Parse(time.DateOnly, c.Value.(string))
				t := time.Date(d.Year(), d.Month(), d.Day(), hh, mm, ss, ns, loc)
				out = append(out, Candidate{Value: t, Confidence: c.Confidence * zoneConf})
			}
		}
		return out
	}
	for _, c := range semDateCands(s, h) {
		d, _ := time.Parse(time.DateOnly, c.Value.(string))
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
		out = append(out, Candidate{Value: t, Confidence: c.Confidence * zoneConf * 0.6})
	}
	return out
}

func semParseTimestamp(in any, h Hints) ([]Candidate, error) {
	var out []Candidate
	for _, c := range semInstants(in, h) {
		ts := c.Value.(time.Time)
		zone := "UTC"
		if z, err := semZone(h.Zone); err == nil && h.Zone != "" {
			_, off := ts.Zone()
			if _, want := ts.In(semLoc(z)).Zone(); want == off {
				zone = z
			}
		}
		out = append(out, Candidate{Value: map[string]any{"utc": semUTC(ts), "zone": zone}, Confidence: c.Confidence})
	}
	return out, nil
}

func semParseTime(in any, h Hints) ([]Candidate, error) {
	s, _ := in.(string)
	hh, mm, ss, _, ok := semClock(s)
	if !ok {
		return nil, errors.New("not a time of day")
	}
	zone, _ := semZone(h.Zone)
	return []Candidate{{Value: map[string]any{"time": fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss), "zone": semNull(zone)}, Confidence: 1}}, nil
}

var semDurRe = regexp.MustCompile(`^P(?:([0-9]+)Y)?(?:([0-9]+)M)?(?:([0-9]+)W)?(?:([0-9]+)D)?(?:T(?:([0-9]+)H)?(?:([0-9]+)M)?(?:([0-9]+(?:\.[0-9]+)?)S)?)?$`)

func semDuration(s string) (string, error) {
	m := semDurRe.FindStringSubmatch(s)
	if m == nil || s == "P" || strings.HasSuffix(s, "T") {
		return "", errors.New("must be an ISO 8601 duration")
	}
	n := func(i int) int64 { v, _ := strconv.ParseInt(m[i], 10, 64); return v }
	secs := "0"
	if m[7] != "" {
		secs, _ = semDec(m[7])
	}
	whole, frac, _ := strings.Cut(secs, ".")
	w, _ := strconv.ParseInt(whole, 10, 64)
	return semDurString(n(1), n(2), n(3)*7+n(4), w+n(5)*3600+n(6)*60, frac), nil
}

func semDurString(y, mo, d, w int64, frac string) string {
	var b strings.Builder
	b.WriteString("P")
	for _, p := range []struct {
		n int64
		u string
	}{{y, "Y"}, {mo, "M"}, {d, "D"}} {
		if p.n > 0 {
			fmt.Fprintf(&b, "%d%s", p.n, p.u)
		}
	}
	if w > 0 || frac != "" {
		b.WriteString("T")
		if w >= 3600 {
			fmt.Fprintf(&b, "%dH", w/3600)
		}
		if w%3600 >= 60 {
			fmt.Fprintf(&b, "%dM", w%3600/60)
		}
		if w%60 > 0 || frac != "" {
			fmt.Fprintf(&b, "%d", w%60)
			if frac != "" {
				b.WriteString("." + frac)
			}
			b.WriteString("S")
		}
	}
	if b.Len() == 1 {
		return "PT0S"
	}
	return b.String()
}

func semParseDuration(in any, _ Hints) ([]Candidate, error) {
	s, _ := in.(string)
	if c, err := semDuration(strings.ToUpper(s)); err == nil {
		return []Candidate{{Value: c, Confidence: 1}}, nil
	}
	secs := func(sec string) string {
		d, _ := semDec(sec)
		whole, frac, _ := strings.Cut(d, ".")
		w, _ := strconv.ParseInt(whole, 10, 64)
		return semDurString(0, 0, 0, w, frac)
	}
	if d, err := time.ParseDuration(s); err == nil && d >= 0 {
		return []Candidate{{Value: secs(strconv.FormatFloat(d.Seconds(), 'f', -1, 64)), Confidence: 1}}, nil
	}
	if hh, mm, ss, ns, ok := semClock(s); ok && !strings.ContainsAny(s, "aApP") {
		return []Candidate{{Value: secs(fmt.Sprintf("%d.%09d", hh*3600+mm*60+ss, ns)), Confidence: 0.9}}, nil
	}
	return nil, errors.New("not a duration")
}

func semEndpoint(s string) (time.Time, bool) {
	if d, err := time.Parse(time.DateOnly, s); err == nil && len(s) == 10 {
		return d, true
	}
	return semParseUTC(s)
}

func semCheckInterval(v any) error {
	m, _ := v.(map[string]any)
	var ts [2]time.Time
	for i, k := range []string{"start", "end"} {
		if s := semS(m, k); s != "" {
			var ok bool
			if ts[i], ok = semEndpoint(s); !ok {
				return fmt.Errorf("%s must be a date or UTC instant", k)
			}
		}
	}
	if m["start"] == nil && m["end"] == nil {
		return errors.New("needs a start or an end")
	}
	if !ts[0].IsZero() && !ts[1].IsZero() && ts[1].Before(ts[0]) {
		return errors.New("end is before start")
	}
	return nil
}

func semParseInterval(in any, h Hints) ([]Candidate, error) {
	point := func(s string) (any, float64, bool) {
		if s == "" || s == ".." {
			return nil, 1, true
		}
		if c := semDateCands(s, h); len(c) > 0 && !semDateTime.MatchString(s) && !strings.Contains(s, "T") {
			return c[0].Value, c[0].Confidence, true
		}
		if c := semInstants(s, h); len(c) > 0 {
			return semUTC(c[0].Value.(time.Time)), c[0].Confidence, true
		}
		return nil, 0, false
	}
	var a, b string
	switch x := in.(type) {
	case string:
		var ok bool
		if a, b, ok = strings.Cut(x, "/"); !ok {
			if a, b, ok = strings.Cut(x, "--"); !ok {
				return nil, errors.New("interval must be start/end")
			}
		}
	case map[string]any:
		a, b = semFirst(x, "start", "from"), semFirst(x, "end", "to", "until")
	default:
		return nil, errors.New("interval must be start/end")
	}
	s, cs, ok1 := point(strings.TrimSpace(a))
	e, ce, ok2 := point(strings.TrimSpace(b))
	if !ok1 || !ok2 {
		return nil, errors.New("bad endpoint")
	}
	return []Candidate{{Value: map[string]any{"start": s, "end": e}, Confidence: cs * ce}}, nil
}

var semRRuleParts = sync.OnceValues(func() (order []string, re map[string]*regexp.Regexp) {
	re = map[string]*regexp.Regexp{}
	list := func(item string) string { return item + "(," + item + ")*" }
	for _, p := range [][2]string{
		{"FREQ", "SECONDLY|MINUTELY|HOURLY|DAILY|WEEKLY|MONTHLY|YEARLY"},
		{"UNTIL", "[0-9]{8}(T[0-9]{6}Z?)?"},
		{"COUNT", "[1-9][0-9]*"},
		{"INTERVAL", "[1-9][0-9]*"},
		{"BYSECOND", list("([1-5]?[0-9]|60)")},
		{"BYMINUTE", list("[1-5]?[0-9]")},
		{"BYHOUR", list("(1?[0-9]|2[0-3])")},
		{"BYDAY", list("([+-]?([1-9]|[1-4][0-9]|5[0-3]))?(MO|TU|WE|TH|FR|SA|SU)")},
		{"BYMONTHDAY", list("[+-]?([1-9]|[12][0-9]|3[01])")},
		{"BYYEARDAY", list("[+-]?([1-9][0-9]?|[12][0-9]{2}|3[0-5][0-9]|36[0-6])")},
		{"BYWEEKNO", list("[+-]?([1-9]|[1-4][0-9]|5[0-3])")},
		{"BYMONTH", list("([1-9]|1[0-2])")},
		{"BYSETPOS", list("[+-]?([1-9][0-9]?|[12][0-9]{2}|3[0-5][0-9]|36[0-6])")},
		{"WKST", "MO|TU|WE|TH|FR|SA|SU"},
	} {
		order = append(order, p[0])
		re[p[0]] = regexp.MustCompile("^(" + p[1] + ")$")
	}
	return order, re
})

func semRRule(s string) (string, error) {
	order, re := semRRuleParts()
	parts := map[string]string{}
	for _, p := range strings.Split(strings.TrimPrefix(strings.ToUpper(s), "RRULE:"), ";") {
		k, v, _ := strings.Cut(p, "=")
		if re[k] == nil || parts[k] != "" || !re[k].MatchString(v) {
			return "", fmt.Errorf("bad rule part %q", p)
		}
		parts[k] = v
	}
	if parts["FREQ"] == "" || (parts["UNTIL"] != "" && parts["COUNT"] != "") {
		return "", errors.New("needs FREQ, and not both UNTIL and COUNT")
	}
	var out []string
	for _, k := range order {
		if v := parts[k]; v != "" {
			out = append(out, k+"="+v)
		}
	}
	return strings.Join(out, ";"), nil
}

var semDollars = map[string]bool{"USD": true, "CAD": true, "AUD": true, "NZD": true, "SGD": true, "HKD": true, "MXN": true}

func semCurrency(s string, h Hints) (string, error) {
	switch s {
	case "€":
		return "EUR", nil
	case "£":
		return "GBP", nil
	case "¥":
		return "JPY", nil
	case "US$":
		return "USD", nil
	case "$":
		if c := strings.ToUpper(h.Currency); semDollars[c] {
			return c, nil
		}
		return "USD", nil
	}
	s = strings.ToUpper(s)
	if u, err := currency.ParseISO(s); err != nil || len(s) != 3 || u.String() != s {
		return "", errors.New("not an ISO 4217 currency")
	}
	return s, nil
}

func semMinor(cur string) int {
	u, err := currency.ParseISO(cur)
	if err != nil {
		return 2
	}
	scale, _ := currency.Standard.Rounding(u)
	return scale
}

func semAmount(d, cur string) (string, bool) {
	minor := semMinor(cur)
	i, f, _ := strings.Cut(d, ".")
	if len(f) > minor || d == "" {
		return "", false
	}
	if minor == 0 {
		return i, true
	}
	return i + "." + f + strings.Repeat("0", minor-len(f)), true
}

var semMoneyCur = regexp.MustCompile(`^\s*([A-Za-z]{3}|US\$|[€£¥$])\s*|\s*([A-Za-z]{3}|[€£¥$])\s*$`)

func semParseMoney(in any, h Hints) ([]Candidate, error) {
	var raw any = in
	cur, curConf := "", 1.0
	if m, ok := in.(map[string]any); ok {
		raw, cur = m["amount"], semFirst(m, "currency")
		if raw == nil {
			return nil, errors.New("missing amount")
		}
	}
	var nums []Candidate
	if s, ok := semNumStr(raw); ok {
		if d, ok := semDec(s); ok {
			nums = []Candidate{{Value: d, Confidence: 1}}
		}
	} else if s, ok := raw.(string); ok {
		s = strings.TrimSpace(s)
		sign := ""
		if strings.HasPrefix(s, "-") {
			sign, s = "-", strings.TrimSpace(s[1:])
		}
		if g := semMoneyCur.FindStringSubmatch(s); g != nil && cur == "" {
			sym := g[1] + g[2]
			if sym == "$" && !semDollars[strings.ToUpper(h.Currency)] {
				curConf = 0.8
			}
			cur = sym
			s = strings.TrimSpace(semMoneyCur.ReplaceAllString(s, ""))
		}
		nums = semNumber(sign+s, h)
	}
	if cur == "" {
		cur, curConf = h.Currency, 0.9
	}
	c, err := semCurrency(cur, h)
	if err != nil {
		return nil, err
	}
	var out []Candidate
	for _, n := range nums {
		if a, ok := semAmount(n.Value.(string), c); ok {
			out = append(out, Candidate{Value: map[string]any{"amount": a, "currency": c}, Confidence: n.Confidence * curConf})
		}
	}
	if len(out) == 1 {
		out[0].Confidence = curConf
	}
	return out, nil
}

func semMod97(s string) int {
	r := 0
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			r = (r*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			r = (r*100 + int(c-'A'+10)) % 97
		default:
			return -1
		}
	}
	return r
}

func semLuhn(digits string) bool {
	if !semAllDigits(digits) {
		return false
	}
	sum := 0
	for i := range digits {
		d := int(digits[len(digits)-1-i] - '0')
		if i%2 == 1 {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

func semGS1(body string) byte {
	sum := 0
	for i := range body {
		d := int(body[len(body)-1-i] - '0')
		if i%2 == 0 {
			d *= 3
		}
		sum += d
	}
	return byte('0' + (10-sum%10)%10)
}

func semABACheck(s string) bool {
	if len(s) != 9 || !semAllDigits(s) {
		return false
	}
	d := func(i int) int { return int(s[i] - '0') }
	return (3*(d(0)+d(3)+d(6))+7*(d(1)+d(4)+d(7))+d(2)+d(5)+d(8))%10 == 0
}

func semISIN(s string) bool {
	var b strings.Builder
	for _, c := range s {
		if c >= 'A' {
			b.WriteString(strconv.Itoa(int(c - 'A' + 10)))
		} else {
			b.WriteRune(c)
		}
	}
	return semLuhn(b.String())
}

func semSSN(s string) bool {
	return len(s) == 9 && semAllDigits(s) && s[:3] != "000" && s[:3] != "666" && s[0] != '9' && s[3:5] != "00" && s[5:] != "0000"
}

var semBICRe = regexp.MustCompile(`^[A-Z]{4}([A-Z]{2})[A-Z0-9]{2}([A-Z0-9]{3})?$`)

func semBIC(s string) (string, error) {
	m := semBICRe.FindStringSubmatch(s)
	if m == nil {
		return "", errors.New("must be an 8 or 11 character BIC")
	}
	if _, err := semCountry(m[1]); err != nil {
		return "", err
	}
	return strings.TrimSuffix(s, "XXX"), nil
}

func semISBN13(s string) (string, error) {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "ISBN"), ":")
	if len(s) != 10 {
		return s, nil
	}
	sum := 0
	for i := 0; i < 10; i++ {
		d := int(s[i] - '0')
		if i == 9 && s[i] == 'X' {
			d = 10
		} else if s[i] < '0' || s[i] > '9' {
			return "", errors.New("bad ISBN-10")
		}
		sum += d * (10 - i)
	}
	if sum%11 != 0 {
		return "", errors.New("ISBN-10 check digit does not match")
	}
	s = "978" + s[:9]
	return s + string(semGS1(s)), nil
}

func semCardBrand(n string) string {
	p := func(k int) int { v, _ := strconv.Atoi(n[:k]); return v }
	switch {
	case n[0] == '4':
		return "visa"
	case p(2) >= 51 && p(2) <= 55, p(4) >= 2221 && p(4) <= 2720:
		return "mastercard"
	case p(2) == 34, p(2) == 37:
		return "amex"
	case p(4) == 6011, p(2) == 65, p(3) >= 644 && p(3) <= 649:
		return "discover"
	case p(4) >= 3528 && p(4) <= 3589:
		return "jcb"
	case p(2) == 36, p(2) == 38, p(2) == 39, p(3) >= 300 && p(3) <= 305:
		return "diners"
	case p(2) == 62:
		return "unionpay"
	case p(2) == 50, p(2) >= 56 && p(2) <= 69:
		return "maestro"
	}
	return "unknown"
}

func semParseCard(in any, _ Hints) ([]Candidate, error) {
	s, _ := in.(string)
	n := semClean(s)
	if len(n) < 12 || len(n) > 19 || !semLuhn(n) {
		return nil, errors.New("not a valid card number (Luhn)")
	}
	sum := sha256.Sum256([]byte(n))
	return []Candidate{{Value: map[string]any{"token": hex.EncodeToString(sum[:]), "brand": semCardBrand(n), "last4": n[len(n)-4:]}, Confidence: 1}}, nil
}

var (
	semNINORe     = regexp.MustCompile(`^[A-CEGHJ-PR-TW-Z][A-CEGHJ-NPR-TW-Z][0-9]{6}[A-D]$`)
	semPassportRe = regexp.MustCompile(`^[A-Z0-9]{6,9}$`)
)

var semNationals = map[string]struct {
	scheme string
	check  func(string) bool
}{
	"US": {"ssn", semSSN},
	"GB": {"nino", func(s string) bool {
		return semNINORe.MatchString(s) && !strings.Contains(" BG GB NK KN TN NT ZZ ", " "+s[:2]+" ")
	}},
}

func semCountryValue(in any, h Hints, keys ...string) (country, value string, conf float64, err error) {
	conf = 1
	switch x := in.(type) {
	case map[string]any:
		country, value = semFirst(x, "country", "issuing_country"), semFirst(x, keys...)
	case string:
		country, value, conf = semHintCountry(h), x, 0.8
	}
	country, err = semCountry(country)
	return country, semClean(value), conf, err
}

func semParseNational(in any, h Hints) ([]Candidate, error) {
	c, val, conf, err := semCountryValue(in, h, "value", "number")
	if err != nil {
		return nil, err
	}
	return []Candidate{{Value: map[string]any{"country": c, "scheme": semNationals[c].scheme, "value": val}, Confidence: conf}}, nil
}

func semParsePassport(in any, h Hints) ([]Candidate, error) {
	c, n, conf, err := semCountryValue(in, h, "number")
	if err != nil {
		return nil, err
	}
	return []Candidate{{Value: map[string]any{"country": c, "number": n}, Confidence: conf}}, nil
}

var semIDNA = idna.New(idna.MapForLookup(), idna.BidiRule(), idna.VerifyDNSLength(true), idna.StrictDomainName(true))

func semDomain(s string) (string, error) {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	a, err := semIDNA.ToASCII(s)
	if err != nil || !strings.Contains(a, ".") {
		return "", errors.New("not a valid domain name")
	}
	if _, err := netip.ParseAddr(a); err == nil {
		return "", errors.New("an IP address is not a domain")
	}
	return a, nil
}

func semParsePhone(in any, h Hints) ([]Candidate, error) {
	s, ok := in.(string)
	if !ok {
		s, _ = semNumStr(in)
	}
	region, conf := "", 1.0
	if strings.HasPrefix(s, "00") {
		s = "+" + s[2:]
	}
	if !strings.HasPrefix(s, "+") {
		if region = strings.ToUpper(h.Country); region != "" {
			conf = 0.9
		} else if region = semHintCountry(h); region != "" {
			conf = 0.7
		} else {
			return nil, errors.New("no country code and no country hint")
		}
	}
	n, err := phonenumbers.Parse(s, region)
	if err != nil || !phonenumbers.IsValidNumber(n) {
		return nil, errors.New("not a valid phone number")
	}
	return []Candidate{{Value: phonenumbers.Format(n, phonenumbers.E164), Confidence: conf}}, nil
}

func semParseAddress(in any, h Hints) ([]Candidate, error) {
	m, ok := in.(map[string]any)
	if !ok {
		return nil, errors.New("address must be structured")
	}
	c, err := semCountry(semFirst(m, "country"))
	if err != nil {
		if c, err = semCountry(semHintCountry(h)); err != nil {
			return nil, errors.New("address needs a country")
		}
	}
	var lines []any
	switch x := m["lines"].(type) {
	case []any:
		for _, l := range x {
			if s, ok := l.(string); ok && strings.TrimSpace(s) != "" {
				lines = append(lines, strings.TrimSpace(s))
			}
		}
	case string:
		for _, l := range strings.Split(x, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				lines = append(lines, l)
			}
		}
	}
	if s := semFirst(m, "street", "line1"); len(lines) == 0 && s != "" {
		lines = []any{s}
	}
	var region, postcode string
	if r := semFirst(m, "region", "state", "province"); r != "" {
		if region, err = semRegion(r, Hints{Country: c}); err != nil {
			return nil, err
		}
	}
	if p := semFirst(m, "postcode", "postal_code", "zip"); p != "" {
		if postcode, err = semPostcode(p, c); err != nil {
			return nil, err
		}
	}
	return []Candidate{{Value: map[string]any{
		"lines": lines, "locality": semNull(semFirst(m, "locality", "city", "town")),
		"region": semNull(region), "postcode": semNull(postcode), "country": c,
	}, Confidence: 1}}, nil
}

var semParticles = map[string]bool{"van": true, "von": true, "der": true, "den": true, "de": true, "del": true, "della": true, "di": true, "da": true, "du": true, "la": true, "le": true, "zu": true, "dos": true, "das": true}

func semParseName(in any, _ Hints) ([]Candidate, error) {
	if m, ok := in.(map[string]any); ok {
		g, p, f := semFirst(m, "given"), semFirst(m, "prefixes", "prefix"), semFirst(m, "family")
		full := semFirst(m, "full")
		if full == "" {
			full = strings.Join(strings.Fields(g+" "+p+" "+f), " ")
		}
		return []Candidate{{Value: map[string]any{"full": full, "given": semNull(g), "family": semNull(f), "prefixes": semNull(p)}, Confidence: 1}}, nil
	}
	s, _ := in.(string)
	full := strings.Join(strings.Fields(norm.NFC.String(s)), " ")
	if full == "" {
		return nil, errors.New("empty name")
	}
	var given, family []string
	conf := 0.7
	if fam, giv, ok := strings.Cut(full, ","); ok {
		family, given, conf = strings.Fields(fam), strings.Fields(giv), 0.8
	} else if w := strings.Fields(full); len(w) > 1 {
		given, family = w[:len(w)-1], w[len(w)-1:]
	} else {
		given = w
	}
	var pre []string
	for len(given) > 1 && semParticles[strings.ToLower(given[len(given)-1])] {
		pre = append([]string{given[len(given)-1]}, pre...)
		given = given[:len(given)-1]
	}
	return []Candidate{{Value: map[string]any{
		"full": full, "given": semNull(strings.Join(given, " ")),
		"family": semNull(strings.Join(family, " ")), "prefixes": semNull(strings.Join(pre, " ")),
	}, Confidence: conf}}, nil
}

var semDefaultPorts = map[string]string{"http": "80", "https": "443", "ws": "80", "wss": "443", "ftp": "21"}

func semURL(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", errors.New("not a URL")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	def, ok := semDefaultPorts[u.Scheme]
	if !ok || u.Host == "" || u.Opaque != "" {
		return "", errors.New("URL needs an http(s), ws(s) or ftp scheme and a host")
	}
	host, port := u.Hostname(), u.Port()
	if a, err := netip.ParseAddr(host); err == nil {
		host = a.String()
	} else if host, err = semIDNA.ToASCII(host); err != nil || host == "" {
		return "", errors.New("bad URL host")
	}
	if port == def {
		port = ""
	}
	switch {
	case port != "":
		u.Host = net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		u.Host = "[" + host + "]"
	default:
		u.Host = host
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}

func semParseURL(in any, _ Hints) ([]Candidate, error) {
	s, _ := in.(string)
	if c, err := semURL(s); err == nil {
		return []Candidate{{Value: c, Confidence: 1}}, nil
	}
	host, _, _ := strings.Cut(s, "/")
	if strings.Contains(s, "://") || semMust(semDomain(host)) == "" {
		return nil, errors.New("not a URL")
	}
	c, err := semURL("https://" + s)
	return []Candidate{{Value: c, Confidence: 0.8}}, err
}

var semCountryNames = sync.OnceValue(func() map[string]string {
	m := map[string]string{}
	namers := []display.Namer{display.English.Regions()}
	for a := 'A'; a <= 'Z'; a++ {
		for b := 'A'; b <= 'Z'; b++ {
			code := string([]rune{a, b})
			r, err := language.ParseRegion(code)
			if err != nil || !r.IsCountry() || r.String() != code {
				continue
			}
			for _, n := range namers {
				if name := n.Name(r); name != "" {
					m[strings.ToLower(name)] = code
				}
			}
		}
	}
	return m
})

func semCountry(s string) (string, error) {
	s = strings.TrimSpace(s)
	if c, ok := semCountryNames()[strings.ToLower(s)]; ok {
		return c, nil
	}
	if len(s) >= 2 && len(s) <= 3 {
		if r, err := language.ParseRegion(strings.ToUpper(s)); err == nil && r.IsCountry() && len(r.String()) == 2 {
			return r.String(), nil
		}
	}
	return "", fmt.Errorf("%q is not an ISO 3166-1 country", s)
}

var semSubdivisions = map[string]string{
	"US": "AL AK AZ AR CA CO CT DE FL GA HI ID IL IN IA KS KY LA ME MD MA MI MN MS MO MT NE NV NH NJ NM NY NC ND OH OK OR PA RI SC SD TN TX UT VT VA WA WV WI WY DC AS GU MP PR UM VI",
}

var semRegionRe = regexp.MustCompile(`^([A-Z]{2})-([A-Z0-9]{1,3})$`)

func semRegion(s string, h Hints) (string, error) {
	s = strings.ToUpper(strings.NewReplacer(" ", "-", "_", "-").Replace(strings.TrimSpace(s)))
	if !semRegionRe.MatchString(s) && h.Country != "" {
		s = strings.ToUpper(h.Country) + "-" + s
	}
	m := semRegionRe.FindStringSubmatch(s)
	if m == nil {
		return "", errors.New("not an ISO 3166-2 code")
	}
	if _, err := semCountry(m[1]); err != nil {
		return "", err
	}
	if list, ok := semSubdivisions[m[1]]; ok && !strings.Contains(" "+list+" ", " "+m[2]+" ") {
		return "", fmt.Errorf("unknown subdivision %s", s)
	}
	return s, nil
}

var semPostcodes = map[string]struct {
	re  *regexp.Regexp
	fmt func(m []string) string
}{
	"US": {regexp.MustCompile(`^([0-9]{5})(?:-?([0-9]{4}))?$`), func(m []string) string { return strings.TrimSuffix(m[1]+"-"+m[2], "-") }},
	"GB": {regexp.MustCompile(`^([A-Z]{1,2}[0-9][A-Z0-9]?)\s?([0-9][A-Z]{2})$`), func(m []string) string { return m[1] + " " + m[2] }},
}

var semPostcodeRe = regexp.MustCompile(`^[A-Z0-9]+([ -][A-Z0-9]+)*$`)

func semPostcode(s, country string) (string, error) {
	s = strings.ToUpper(strings.Join(strings.Fields(s), " "))
	if r, ok := semPostcodes[country]; ok {
		if m := r.re.FindStringSubmatch(s); m != nil {
			return r.fmt(m), nil
		}
		return "", fmt.Errorf("not a %s postcode", country)
	}
	if len(s) < 2 || len(s) > 10 || !semPostcodeRe.MatchString(s) {
		return "", errors.New("not a postcode")
	}
	return s, nil
}

func semParsePostcode(in any, h Hints) ([]Candidate, error) {
	s, c := "", semHintCountry(h)
	switch x := in.(type) {
	case map[string]any:
		s = semFirst(x, "code", "postcode")
		if cc := semFirst(x, "country"); cc != "" {
			c = cc
		}
	default:
		if s, _ = in.(string); s == "" {
			s, _ = semNumStr(in)
		}
	}
	one := func(c string, conf float64) []Candidate {
		p, err := semPostcode(s, c)
		if err != nil {
			return nil
		}
		return []Candidate{{Value: map[string]any{"country": c, "code": p}, Confidence: conf}}
	}
	if c != "" {
		cc, err := semCountry(c)
		if err != nil {
			return nil, err
		}
		return one(cc, 1), nil
	}
	var out []Candidate
	for _, c := range slices.Sorted(maps.Keys(semPostcodes)) {
		out = append(out, one(c, 0)...)
	}
	if len(out) == 0 {
		return nil, errors.New("postcode needs a country")
	}
	for i := range out {
		out[i].Confidence = 1 / float64(len(out))
	}
	return out, nil
}

var semGeoRe = regexp.MustCompile(`^([-+]?[0-9]+(?:\.[0-9]+)?)\s*[,;\s]\s*([-+]?[0-9]+(?:\.[0-9]+)?)$`)

func semParseGeo(in any, _ Hints) ([]Candidate, error) {
	var a, b string
	switch x := in.(type) {
	case string:
		m := semGeoRe.FindStringSubmatch(x)
		if m == nil {
			return nil, errors.New("want 'lat, lon'")
		}
		a, b = m[1], m[2]
	case map[string]any:
		a, b = semFirst(x, "lat", "latitude"), semFirst(x, "lon", "lng", "long", "longitude")
	}
	lat, err1 := strconv.ParseFloat(a, 64)
	lon, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil {
		return nil, errors.New("want latitude and longitude")
	}
	r := func(f float64) float64 { return math.Round(f*1e7) / 1e7 }
	return []Candidate{{Value: map[string]any{"lat": r(lat), "lon": r(lon)}, Confidence: 1}}, nil
}

func semLocale(s string) (string, error) {
	t, err := language.Parse(strings.ReplaceAll(s, "_", "-"))
	if err != nil || t == language.Und {
		return "", errors.New("not a BCP 47 locale")
	}
	return t.String(), nil
}

type semUnit struct {
	base           string
	factor, offset float64
	disp           string
}

var semUnits = map[string]semUnit{
	"m": {"m", 1, 0, "m"}, "km": {"m", 1e3, 0, "km"}, "cm": {"m", 1e-2, 0, "cm"}, "mm": {"m", 1e-3, 0, "mm"}, "um": {"m", 1e-6, 0, "µm"}, "nm": {"m", 1e-9, 0, "nm"},
	"[in_i]": {"m", 0.0254, 0, "in"}, "[ft_i]": {"m", 0.3048, 0, "ft"}, "[yd_i]": {"m", 0.9144, 0, "yd"}, "[mi_i]": {"m", 1609.344, 0, "mi"}, "[nmi_i]": {"m", 1852, 0, "nmi"},
	"kg": {"kg", 1, 0, "kg"}, "g": {"kg", 1e-3, 0, "g"}, "mg": {"kg", 1e-6, 0, "mg"}, "ug": {"kg", 1e-9, 0, "µg"}, "t": {"kg", 1e3, 0, "t"},
	"[lb_av]": {"kg", 0.45359237, 0, "lb"}, "[oz_av]": {"kg", 0.028349523125, 0, "oz"}, "[stone_av]": {"kg", 6.35029318, 0, "st"},
	"m2": {"m2", 1, 0, "m²"}, "cm2": {"m2", 1e-4, 0, "cm²"}, "km2": {"m2", 1e6, 0, "km²"}, "har": {"m2", 1e4, 0, "ha"},
	"m3": {"m3", 1, 0, "m³"}, "cm3": {"m3", 1e-6, 0, "cm³"}, "L": {"m3", 1e-3, 0, "L"}, "dL": {"m3", 1e-4, 0, "dL"}, "cL": {"m3", 1e-5, 0, "cL"}, "mL": {"m3", 1e-6, 0, "mL"},
	"[gal_us]": {"m3", 0.003785411784, 0, "gal"}, "[qt_us]": {"m3", 0.000946352946, 0, "qt"}, "[pt_us]": {"m3", 0.000473176473, 0, "pt"}, "[foz_us]": {"m3", 2.95735295625e-5, 0, "fl oz"}, "[gal_br]": {"m3", 0.00454609, 0, "gal (UK)"},
	"K": {"K", 1, 0, "K"}, "Cel": {"K", 1, 273.15, "°C"}, "[degF]": {"K", 5.0 / 9, 459.67 * 5 / 9, "°F"},
	"s": {"s", 1, 0, "s"}, "ms": {"s", 1e-3, 0, "ms"}, "min": {"s", 60, 0, "min"}, "h": {"s", 3600, 0, "h"}, "d": {"s", 86400, 0, "d"}, "wk": {"s", 604800, 0, "wk"},
	"J": {"J", 1, 0, "J"}, "kJ": {"J", 1e3, 0, "kJ"}, "MJ": {"J", 1e6, 0, "MJ"}, "cal": {"J", 4.184, 0, "cal"}, "kcal": {"J", 4184, 0, "kcal"}, "W.h": {"J", 3600, 0, "Wh"}, "kW.h": {"J", 3.6e6, 0, "kWh"},
	"W": {"W", 1, 0, "W"}, "kW": {"W", 1e3, 0, "kW"}, "MW": {"W", 1e6, 0, "MW"},
	"Pa": {"Pa", 1, 0, "Pa"}, "kPa": {"Pa", 1e3, 0, "kPa"}, "bar": {"Pa", 1e5, 0, "bar"}, "mbar": {"Pa", 100, 0, "mbar"}, "[psi]": {"Pa", 6894.757293168, 0, "psi"},
	"m/s": {"m/s", 1, 0, "m/s"}, "km/h": {"m/s", 1 / 3.6, 0, "km/h"}, "[mi_i]/h": {"m/s", 0.44704, 0, "mph"},
}

var semUnitAliases = map[string]string{
	"meter": "m", "meters": "m", "metre": "m", "metres": "m", "kilometer": "km", "kilometers": "km", "µm": "um",
	"in": "[in_i]", "inch": "[in_i]", "inches": "[in_i]", `"`: "[in_i]", "ft": "[ft_i]", "foot": "[ft_i]", "feet": "[ft_i]", "'": "[ft_i]",
	"yd": "[yd_i]", "mi": "[mi_i]", "mile": "[mi_i]", "miles": "[mi_i]", "nmi": "[nmi_i]",
	"kilo": "kg", "kilos": "kg", "gram": "g", "grams": "g", "µg": "ug", "tonne": "t", "tonnes": "t",
	"lb": "[lb_av]", "lbs": "[lb_av]", "pound": "[lb_av]", "pounds": "[lb_av]", "oz": "[oz_av]", "ounce": "[oz_av]", "ounces": "[oz_av]", "st": "[stone_av]",
	"m²": "m2", "cm²": "cm2", "km²": "km2", "ha": "har", "hectare": "har", "m³": "m3", "cm³": "cm3", "cc": "cm3",
	"l": "L", "liter": "L", "litre": "L", "liters": "L", "litres": "L", "ml": "mL", "cl": "cL", "dl": "dL",
	"gal": "[gal_us]", "gallon": "[gal_us]", "gallons": "[gal_us]", "qt": "[qt_us]", "pt": "[pt_us]", "fl oz": "[foz_us]",
	"°c": "Cel", "c": "Cel", "celsius": "Cel", "degc": "Cel", "°f": "[degF]", "f": "[degF]", "fahrenheit": "[degF]", "kelvin": "K",
	"sec": "s", "second": "s", "seconds": "s", "minute": "min", "minutes": "min", "hr": "h", "hour": "h", "hours": "h", "day": "d", "days": "d", "week": "wk", "weeks": "wk",
	"kj": "kJ", "kcal": "kcal", "wh": "W.h", "kwh": "kW.h", "w": "W", "kw": "kW", "pa": "Pa", "kpa": "kPa", "psi": "[psi]",
	"kmh": "km/h", "kph": "km/h", "mph": "[mi_i]/h",
}

var semQtyRe = regexp.MustCompile(`^([-+−]?[0-9][0-9.,\s]*?)\s*([^0-9\s.,].*)$`)

func semParseQuantity(in any, h Hints) ([]Candidate, error) {
	var num any
	var unit string
	switch x := in.(type) {
	case string:
		m := semQtyRe.FindStringSubmatch(x)
		if m == nil {
			return nil, errors.New("want a number and a unit")
		}
		num, unit = m[1], strings.TrimSpace(m[2])
	case map[string]any:
		num, unit = x["value"], semFirst(x, "unit")
	}
	code := unit
	if _, ok := semUnits[unit]; !ok {
		if code, ok = semUnitAliases[strings.ToLower(unit)]; !ok {
			return nil, fmt.Errorf("unknown unit %q", unit)
		}
	}
	var nums []Candidate
	if s, ok := semNumStr(num); ok {
		nums = []Candidate{{Value: s, Confidence: 1}}
	} else if s, ok := num.(string); ok {
		nums = semNumber(s, h)
	}
	u := semUnits[code]
	var out []Candidate
	for _, n := range nums {
		f, _ := strconv.ParseFloat(n.Value.(string), 64)
		out = append(out, Candidate{Value: map[string]any{"value": semRound12(f*u.factor + u.offset), "unit": u.base, "original_unit": code}, Confidence: n.Confidence})
	}
	return out, nil
}

func semUUID(s string) (string, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return "", errors.New("not a UUID")
	}
	return id.String(), nil
}

func semIP(s string) (string, error) {
	a, err := netip.ParseAddr(strings.Trim(s, "[]"))
	if err != nil {
		return "", errors.New("not an IP address")
	}
	return a.String(), nil
}

var semMIMETops = " application audio font example image message model multipart text video "

func semMIME(s string) (string, error) {
	mt, params, err := mime.ParseMediaType(s)
	top, _, _ := strings.Cut(mt, "/")
	if err != nil || !strings.Contains(semMIMETops, " "+top+" ") {
		return "", errors.New("not an IANA media type")
	}
	return mime.FormatMediaType(mt, params), nil
}

type semType struct {
	Normalize []string
	Parse     string
	Check     string
	Mask      string
	Secret    bool
}

var semBase = map[string]string{
	"text": "text", "varchar": "text", "bpchar": "text", "int2": "integer", "int4": "integer", "int8": "integer",
	"numeric": "decimal", "float4": "number", "float8": "number", "bool": "boolean", "date": "date", "uuid": "uuid", "timestamptz": "instant",
}

var semLib = map[string]semType{
	"text":       {Normalize: []string{"nfc"}},
	"richtext":   {Normalize: []string{"nfc"}},
	"integer":    {Parse: "number"},
	"number":     {Parse: "number"},
	"decimal":    {Parse: "decimal"},
	"boolean":    {Parse: "boolean"},
	"date":       {Parse: "date", Check: "date"},
	"instant":    {Parse: "instant", Check: "instant"},
	"uuid":       {Normalize: []string{"uuid"}},
	"timestamp":  {Parse: "timestamp", Check: "timestamp"},
	"time":       {Parse: "time", Check: "time"},
	"duration":   {Parse: "duration", Check: "duration"},
	"interval":   {Parse: "interval", Check: "interval"},
	"timezone":   {Normalize: []string{"timezone"}},
	"recurrence": {Normalize: []string{"rrule"}},
	"money":      {Parse: "money", Check: "money"},
	"currency":   {Parse: "currency", Check: "currency"},
	"percentage": {Parse: "percentage"},
	"bic":        {Normalize: []string{"strip_separators", "upper", "bic"}},
	"card":       {Parse: "card", Mask: "card", Secret: true},
	"us_routing": {Normalize: []string{"strip_separators"}, Check: "aba"},
	"lei":        {Normalize: []string{"strip_separators", "upper"}, Check: "lei"},
	"isin":       {Normalize: []string{"strip_separators", "upper"}, Check: "isin"},
	"us_ein":     {Normalize: []string{"strip_separators", "ein_dash"}},
	"us_ssn":     {Normalize: []string{"strip_separators"}, Check: "ssn_ranges", Mask: "mask_ssn", Secret: true},
	"national":   {Parse: "national", Check: "national", Mask: "national", Secret: true},
	"passport":   {Parse: "passport", Check: "passport", Mask: "passport", Secret: true},
	"email":      {Normalize: []string{"mail_address", "lower_domain"}},
	"phone":      {Parse: "phone", Check: "phone"},
	"address":    {Parse: "address", Check: "address"},
	"name":       {Parse: "name", Check: "name"},
	"url":        {Parse: "url", Check: "url"},
	"country":    {Normalize: []string{"country"}},
	"region":     {Parse: "region", Check: "region"},
	"postcode":   {Parse: "postcode", Check: "postcode"},
	"geo":        {Parse: "geo", Check: "geo"},
	"locale":     {Normalize: []string{"locale"}},
	"quantity":   {Parse: "quantity", Check: "quantity"},
	"gtin":       {Normalize: []string{"strip_separators", "gtin14"}, Check: "gtin"},
	"isbn":       {Normalize: []string{"strip_separators", "upper", "isbn13"}, Check: "isbn"},
	"ip":         {Normalize: []string{"ip"}},
	"domain":     {Normalize: []string{"punycode"}},
	"mime":       {Normalize: []string{"mime"}},
}
