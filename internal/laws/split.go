package laws

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Article is one numbered provision of a law. Num is canonical ("23", "4bis",
// "48-3" for Korean 제48조의3 or Japanese 第48条の3) so lookups don't depend on
// the source language's numbering style.
type Article struct {
	Num     string
	Heading string
	Body    string
}

// headingRE finds article headings at a line start, per source language. Group 1
// (and optional group 2, a sub-number) feed NormalizeNum. A source can override
// this with its own `split` regex in the manifest.
var headingRE = map[string]string{
	"en": `(?m)^[ \t]*(?:Article|ARTICLE|Art\.)[ \t]+(\d+[a-z]*)\b`,
	"zh": `(?m)^[ \t　]*第([一二三四五六七八九十百千零〇两\d]+)条`,
	"ja": `(?m)^[ \t　]*第([一二三四五六七八九十百千\d]+)条(?:の([一二三四五六七八九十\d]+))?`,
	"ko": `(?m)^[ \t]*제(\d+)조(?:의(\d+))?`,
	"es": `(?m)^[ \t]*(?:Art[íi]culo|ART[ÍI]CULO|Art\.)[ \t]*(\d+(?:[ \t]*(?:bis|ter|quater))?)`,
	"pt": `(?m)^[ \t]*(?:Art\.?|Artigo)[ \t]*(\d+(?:-[A-Z])?)`,
	"tr": `(?m)^[ \t]*(?:MADDE|Madde)[ \t]+(\d+)`,
	"ar": `(?m)^[ \t]*(?:المادة|مادة)[ \t]*[(\[]?[ \t]*([0-9٠-٩]+|[^\n:：()\[\]]{2,40})`,
	"vi": `(?m)^[ \t]*Điều[ \t]+(\d+)`,
	"id": `(?m)^[ \t]*Pasal[ \t]+(\d+[A-Z]?)`,
	"ms": `(?m)^[ \t]*(?:Seksyen|Section)[ \t]+(\d+[A-Z]?)`,
	"th": `(?m)^[ \t]*มาตรา[ \t]*([0-9๐-๙]+(?:/[0-9๐-๙]+)?)`,
}

// maxGap bounds how far the numbering may jump between two headings (repealed
// articles leave small gaps).
const maxGap = 10

// Split cuts text into articles. custom, if set, replaces the language default.
// Text before the first heading becomes a "preamble" article; text with no
// recognisable headings comes back as a single "all" article.
func Split(text, lang, custom string, gapLimit int) []Article {
	pat := custom
	if pat == "" {
		pat = headingRE[lang]
	}
	if pat == "" {
		return []Article{{Num: "all", Body: strings.TrimSpace(text)}}
	}
	re := regexp.MustCompile(pat)
	var cands []cut
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		raw := text[m[2]:m[3]]
		if len(m) >= 6 && m[4] >= 0 {
			raw += "-" + text[m[4]:m[5]]
		}
		num := NormalizeNum(raw)
		if v, ok := seqValue(num); ok {
			cands = append(cands, cut{m[0], num, v})
		}
	}
	if gapLimit <= 0 {
		gapLimit = maxGap
	}
	cuts := chain(cands, gapLimit)
	if len(cuts) == 0 {
		return []Article{{Num: "all", Body: strings.TrimSpace(text)}}
	}
	var out []Article
	if pre := strings.TrimSpace(text[:cuts[0].start]); pre != "" {
		out = append(out, Article{Num: "preamble", Body: pre})
	}
	tail := len(text) // annexes after the last article are kept apart from it
	if m := annexRE.FindStringIndex(text[cuts[len(cuts)-1].start:]); m != nil {
		tail = cuts[len(cuts)-1].start + m[0]
	}
	for i, c := range cuts {
		end := tail
		if i+1 < len(cuts) {
			end = cuts[i+1].start
		}
		body := strings.TrimSpace(text[c.start:end])
		heading, _, _ := strings.Cut(body, "\n")
		out = append(out, Article{Num: c.num, Heading: strings.TrimSpace(heading), Body: body})
	}
	if rest := strings.TrimSpace(text[tail:]); rest != "" {
		heading, _, _ := strings.Cut(rest, "\n")
		out = append(out, Article{Num: "annex", Heading: strings.TrimSpace(heading), Body: rest})
	}
	return out
}

// annexRE marks where annexes, schedules or transitory provisions start after
// the last article, so they don't inflate it.
var annexRE = regexp.MustCompile(`(?m)^[ \t]*(?:ANNEX|Annex|SCHEDULE|Schedule|ANEXO|Anexo|TRANSITORIOS|Transitorios|DISPOSICIONES TRANSITORIAS|ملحق|附件)\b`)

type cut struct {
	start int
	num   string
	v     int
}

// chain picks the real headings among the candidates: the longest run in
// document order whose numbers rise by at most maxGap per step (a sub-number
// like "2-2" may follow "2"), preferring the smallest total gap. A
// cross-reference that happens to start a line can't extend the run as well as
// the true next article can, so it drops out.
// shortcut: a TOC and body of equal length tie toward the body (later run); a
// TOC that lists more headings than the body would win, upgrade then.
func chain(c []cut, gapLimit int) []cut {
	if len(c) == 0 {
		return nil
	}
	n := len(c)
	length, gap, prev := make([]int, n), make([]int, n), make([]int, n)
	best := 0
	for i := range c {
		length[i], prev[i] = 1, -1
		for j := max(0, i-400); j < i; j++ { // window: headings are never hundreds of candidates apart
			d := c[i].v - c[j].v
			if d < 0 || d > gapLimit || (d == 0 && subRank(c[i].num) <= subRank(c[j].num)) {
				continue
			}
			l, g := length[j]+1, gap[j]+d
			// on a tie, the later and (for control ids) the deeper candidate wins: a PDF
			// column can print control 1-4-1 after its subcontrol 1-4-1-1, and the
			// subcontrol is the one with text
			// (in a tie l >= 2, so prev[i] is set)
			tie := l == length[i] && g == gap[i] && strings.Count(c[j].num, "-") >= strings.Count(c[prev[i]].num, "-")
			if l > length[i] || (l == length[i] && g < gap[i]) || tie {
				length[i], gap[i], prev[i] = l, g, j
			}
		}
		// ties go to the later run: a table of contents lists the same headings
		// before the body does
		if length[i] > length[best] || (length[i] == length[best] && gap[i] <= gap[best]) {
			best = i
		}
	}
	var out []cut
	for i := best; i >= 0; i = prev[i] {
		out = append(out, c[i])
	}
	slices.Reverse(out)
	return out
}

// subRank orders the variants of one article number: "2" < "2-1" < "2-2", and
// "5" < "5bis" < "5ter".
func subRank(num string) int {
	i := 0
	for i < len(num) && num[i] >= '0' && num[i] <= '9' {
		i++
	}
	rest := num[i:]
	if j := strings.LastIndex(num, "."); j >= 0 { // CFR "314.4": its parts are counted by seqValue
		rest = ""
	}
	switch {
	case rest == "":
		return 0
	case rest[0] == '-':
		n, err := strconv.Atoi(rest[1:])
		if err != nil {
			return 1
		}
		return n
	}
	if r := map[string]int{"bis": 2, "ter": 3, "quater": 4, "quinquies": 5, "sexies": 6}[rest]; r > 0 {
		return r
	}
	return 1 + int(rest[0]-'a') // 5a, 5b, ...
}

// seqValue is the number headings count up by: the leading integer ("48-3" ->
// 48), or for dotted CFR-style sections the last part ("314.4" -> 4).
func seqValue(num string) (int, bool) {
	// control ids such as Saudi NCA "1-2-1" or "4-1-3-2" (domain-subdomain-control-
	// subcontrol): base 100 per level, so the whole id rises in document order.
	// A domain step is a big jump, so such sources need a large max_gap.
	if parts := strings.Split(num, "-"); len(parts) >= 3 && len(parts) <= 4 {
		v, ok := 0, true
		for i := range 4 {
			n := 0
			if i < len(parts) {
				var err error
				n, err = strconv.Atoi(parts[i])
				ok = ok && err == nil && n <= 99
			}
			v = v*100 + n
		}
		if ok {
			return v, true
		}
		// not a control id ("3-2-1bis"): fall through to the leading number
	}
	if i := strings.LastIndex(num, "."); i >= 0 {
		return leadInt(num[i+1:])
	}
	return leadInt(num)
}

// leadInt reads the leading integer of a canonical number ("48-3" -> 48).
func leadInt(num string) (int, bool) {
	i := 0
	for i < len(num) && num[i] >= '0' && num[i] <= '9' {
		i++
	}
	v, err := strconv.Atoi(num[:i])
	return v, err == nil
}

var numPrefix = regexp.MustCompile(`(?i)^(?:article|art[íi]culo|artigo|art\.?|madde|section|sec\.?|s\.|§+|pasal|seksyen|điều|มาตรา|المادة|مادة|第|제)\s*`)

// NormalizeNum turns an article reference in any supported style into the
// canonical form: "Art. 23" -> "23", "第二十三条" -> "23", "제48조의3" -> "48-3",
// "Artículo 5 bis" -> "5bis", "المادة الثالثة والأربعون" -> "43", "มาตรา ๒๔" -> "24".
func NormalizeNum(s string) string {
	s = strings.TrimSpace(s)
	s = numPrefix.ReplaceAllString(s, "")
	// "(5)" as a whole is a heading style; "33(1)" or "5 §2" cites a paragraph of 33 or 5
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "("), ")"))
	if i := strings.IndexAny(s, "(§"); i > 0 {
		s = s[:i]
	}
	s = strings.NewReplacer("条", "", "조", "", "º", "", "°", "", "ª", "", "の", "-", "의", "-", ":", "", "：", "").Replace(s)
	s = strings.TrimSpace(s)
	if v, ok := arabicOrdinal(s); ok {
		return strconv.Itoa(v)
	}
	var b strings.Builder
	var cjk []rune
	flush := func() {
		if len(cjk) > 0 {
			if v, ok := cnNum(string(cjk)); ok {
				b.WriteString(strconv.Itoa(v))
			}
			cjk = cjk[:0]
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r == '.' && i > 0 && i+1 < len(rs) && unicode.IsDigit(rs[i-1]) && unicode.IsDigit(rs[i+1]):
			b.WriteRune('.') // US CFR style "314.4"
		case strings.ContainsRune("一二三四五六七八九十百千零〇两", r):
			cjk = append(cjk, r)
		case unicode.IsDigit(r):
			flush()
			b.WriteRune('0' + rune(digitValue(r)))
		case r == '-' || r == '/':
			flush()
			b.WriteRune('-')
		case unicode.IsLetter(r):
			flush()
			b.WriteRune(unicode.ToLower(r))
		}
	}
	flush()
	out := strings.Trim(b.String(), "-") // "ARTÍCULO 1.-" is article 1
	if ordinalO.MatchString(out) {       // Mexican "Artículo 1o." is article 1
		out = strings.TrimSuffix(out, "o")
	}
	return out
}

var ordinalO = regexp.MustCompile(`^\d+o$`)

// digitValue maps any Unicode decimal digit (Arabic-Indic, Thai, Devanagari...)
// to its value; Unicode lays each script's digits out as a run of ten.
func digitValue(r rune) int {
	for z := r; z >= r-9; z-- {
		if !unicode.IsDigit(z - 1) {
			return int(r - z)
		}
	}
	return int(r - '0')
}

// cnNum parses Chinese/Japanese numerals up to the thousands: 二十三 = 23,
// 一百零五 = 105, 十二 = 12.
func cnNum(s string) (int, bool) {
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	units := map[rune]int{'十': 10, '百': 100, '千': 1000}
	total, cur := 0, -1
	for _, r := range s {
		if d, ok := digits[r]; ok {
			cur = d
		} else if u, ok := units[r]; ok {
			if cur < 0 {
				cur = 1
			}
			total += cur * u
			cur = -1
		} else {
			return 0, false
		}
	}
	if cur > 0 {
		total += cur
	}
	return total, total > 0
}

// arabicOrdinal parses ordinal-word article numbers used in Gulf legislation
// ("الأولى" = 1, "الحادية عشرة" = 11, "الثالثة والأربعون" = 43). It stops at the
// first word that is not part of a number, so trailing heading text is ignored.
func arabicOrdinal(s string) (int, bool) {
	units := map[string]int{"اول": 1, "اولي": 1, "حادي": 1, "حاديه": 1, "ثاني": 2, "ثانيه": 2, "ثالث": 3, "ثالثه": 3,
		"رابع": 4, "رابعه": 4, "خامس": 5, "خامسه": 5, "سادس": 6, "سادسه": 6, "سابع": 7, "سابعه": 7,
		"ثامن": 8, "ثامنه": 8, "تاسع": 9, "تاسعه": 9, "عاشر": 10, "عاشره": 10, "عشر": 10, "عشره": 10,
		"عشرون": 20, "عشرين": 20, "ثلاثون": 30, "ثلاثين": 30, "اربعون": 40, "اربعين": 40, "خمسون": 50, "خمسين": 50,
		"ستون": 60, "ستين": 60, "سبعون": 70, "سبعين": 70, "ثمانون": 80, "ثمانين": 80, "تسعون": 90, "تسعين": 90,
		"مائه": 100, "مئه": 100}
	total, n := 0, 0
	for _, w := range strings.Fields(normArabic(s)) {
		w = strings.TrimPrefix(w, "و")
		w = strings.TrimPrefix(w, "ال")
		v, ok := units[w]
		if !ok {
			break
		}
		total += v
		n++
	}
	return total, n > 0
}

// normArabic strips short vowels and tatweel and unifies letter variants, so
// searches and ordinal parsing don't depend on how a source spelled a word.
func normArabic(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 0x064B && r <= 0x0652, r == 0x0640, r == 0x0670:
			return -1
		case r == 'أ' || r == 'إ' || r == 'آ':
			return 'ا'
		case r == 'ى':
			return 'ي'
		case r == 'ة':
			return 'ه'
		}
		return r
	}, s)
}
