package laws

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rhaist/marq/internal/untrusted"
)

// Answer serves the law_lookup tool: no arguments lists the corpus; a law alone
// shows its metadata and article numbers; law + article returns that article;
// a query searches (optionally within one law). The law text is fetched from
// third-party sites, so it goes through untrusted.Wrap like any tool output.
func Answer(path, law, article, query string, limit, maxChars int) string {
	st, err := Open(path)
	if errors.Is(err, ErrNoCorpus) {
		return "law corpus not installed at " + path + ". Ask the operator to run `marq laws sync` " +
			"(in the container: `docker exec <container> marq laws sync`), then try again."
	}
	if err != nil {
		return "error: " + err.Error()
	}
	defer st.Close()
	if limit <= 0 {
		limit = 5
	}
	switch {
	case query != "":
		return search(st, law, query, limit, maxChars)
	case law == "":
		return list(st)
	}
	l, err := st.Resolve(law)
	if err != nil {
		return "error: " + err.Error()
	}
	if article == "" {
		nums, _ := st.Nums(l.ID)
		if len(nums) > 300 {
			nums = append(nums[:300], fmt.Sprintf("… (%d more)", len(nums)-300))
		}
		return header(st, l) + "articles: " + strings.Join(nums, ", ")
	}
	a, err := st.Article(l.ID, article)
	if err != nil {
		return header(st, l) + "error: " + err.Error()
	}
	body := a.Body
	if len(body) > maxChars {
		body = clip(body, maxChars) + "\n[truncated]"
	}
	return header(st, l) + untrusted.Wrap(fmt.Sprintf("law %s article %s", l.ID, a.Num), body)
}

// header is marq's own metadata about a law, so it sits outside the fence.
func header(st *Store, l Law) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s", l.ID, l.Title)
	if l.NativeTitle != "" && l.NativeTitle != l.Title {
		fmt.Fprintf(&b, " (%s)", l.NativeTitle)
	}
	fmt.Fprintf(&b, "\njurisdiction %s, language %s, %s text", l.Jurisdiction, l.Lang, l.Kind)
	switch {
	case l.Kind == "reference" && l.Translates == "":
		b.WriteString(" (translation; the official original, not in the corpus, governs)")
	case l.Kind == "reference":
		fmt.Fprintf(&b, " (translation of %s; the official text governs)", l.Translates)
	}
	fmt.Fprintf(&b, "\nversion %s, fetched %s from %s\n", l.VersionDate, l.FetchedAt, l.URL)
	if l.Note != "" {
		b.WriteString("note: " + l.Note + "\n")
	}
	if w := st.Stale(l); w != "" {
		b.WriteString(w + "\n")
	}
	return b.String()
}

func list(st *Store) string {
	all, err := st.List()
	if err != nil {
		return "error: " + err.Error()
	}
	if len(all) == 0 {
		return "the law corpus is empty; run `marq laws sync`"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d laws (id | jurisdiction | lang | kind | version | articles | title):\n", len(all))
	for _, l := range all {
		fmt.Fprintf(&b, "%s | %s | %s | %s | %s | %d | %s\n", l.ID, l.Jurisdiction, l.Lang, l.Kind, l.VersionDate, l.Articles, l.Title)
	}
	return b.String()
}

func search(st *Store, law, query string, limit, maxChars int) string {
	id := ""
	if law != "" {
		l, err := st.Resolve(law)
		if err != nil {
			return "error: " + err.Error()
		}
		id = l.ID
	}
	hits, err := st.Search(query, id, limit)
	if err != nil {
		return "error: " + err.Error()
	}
	if len(hits) == 0 {
		return fmt.Sprintf("no article matches %q. Laws are stored in their original language: search with the "+
			"law's own terms (e.g. 网络安全事件, bildirim, البيانات), or list a law's articles by id.", query)
	}
	var b strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&b, "%s article %s: %s\n", h.LawID, h.Num, h.Snippet)
	}
	out := clip(b.String(), maxChars)
	return untrusted.Wrap("law_lookup search "+query, out) +
		"\nFetch the full text with law_lookup {\"law\": <id>, \"article\": <number>}."
}

// clip cuts s to at most n bytes without splitting a multi-byte character.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
