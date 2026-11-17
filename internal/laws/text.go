package laws

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	dropRE    = regexp.MustCompile(`(?is)<(script|style|head|noscript)\b.*?</(script|style|head|noscript)\s*>`)
	commentRE = regexp.MustCompile(`(?s)<!--.*?-->`)
	blockRE   = regexp.MustCompile(`(?i)<\s*/?\s*(p|div|br|tr|li|h[1-6]|table|section|article|blockquote|dt|dd|title)\b[^>]*>`)
	cellRE    = regexp.MustCompile(`(?i)<\s*/?\s*t[dh]\b[^>]*>`)
	tagRE     = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRE   = regexp.MustCompile(`[ \t\x{00a0}\x{3000}]+`)
	charsetRE = regexp.MustCompile(`(?i)charset\s*=\s*["']?([a-z0-9_-]+)`)
)

// HTMLToText reduces an HTML or XML page to plain text with one block per line,
// which is all the article splitter needs. It is deliberately simple: official
// legal texts are mostly headings and paragraphs.
func HTMLToText(s string) string {
	s = dropRE.ReplaceAllString(s, "")
	s = commentRE.ReplaceAllString(s, "")
	s = blockRE.ReplaceAllString(s, "\n")
	s = cellRE.ReplaceAllString(s, " ")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return tidy(s)
}

// tidy collapses runs of spaces and drops blank lines.
func tidy(s string) string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		if line = strings.TrimSpace(spaceRE.ReplaceAllString(line, " ")); line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// decode returns the body as UTF-8. Latin-1 and Windows-1252 (still common on
// Latin American government sites) are converted; other legacy encodings are
// refused so a garbled text never reaches the corpus.
func decode(body []byte, contentType string) (string, error) {
	if utf8.Valid(body) {
		return string(body), nil
	}
	cs := ""
	if m := charsetRE.FindStringSubmatch(contentType); m != nil {
		cs = strings.ToLower(m[1])
	} else if m := charsetRE.FindSubmatch(body[:min(len(body), 4096)]); m != nil {
		cs = strings.ToLower(string(m[1]))
	}
	switch cs {
	case "iso-8859-1", "latin1", "latin-1", "windows-1252", "cp1252", "us-ascii", "":
		var b strings.Builder
		for _, c := range body {
			if r, ok := cp1252[c]; ok {
				b.WriteRune(r)
			} else {
				b.WriteRune(rune(c))
			}
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("source is %s, not UTF-8; pick a UTF-8 source", cs)
}

// cp1252 holds the Windows-1252 characters that differ from Latin-1 (0x80-0x9F).
var cp1252 = map[byte]rune{0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡', 0x88: 'ˆ',
	0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž', 0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•',
	0x96: '–', 0x97: '—', 0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ'}
