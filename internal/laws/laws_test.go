package laws

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rhaist/marq/internal/config"
)

func TestNormalizeNum(t *testing.T) {
	for in, want := range map[string]string{
		"Art. 23":             "23",
		"Article 4":           "4",
		"第二十三条":               "23",
		"第一百零五条":              "105",
		"第十条":                 "10",
		"第48条の3":              "48-3",
		"제48조의3":              "48-3",
		"Artículo 5 bis":      "5bis",
		"Artículo 1o":         "1",
		"Art. 1º":             "1",
		"MADDE 12":            "12",
		"المادة الأولى":       "1",
		"المادة الحادية عشرة": "11",
		"الثالثة والأربعون":   "43",
		"المادة (٢٠)":         "20",
		"มาตรา ๒๔":            "24",
		"Điều 7":              "7",
		"  12  ":              "12",
		"§ 314.4":             "314.4",
		"ARTÍCULO 1.-":        "1",
		"33(1)":               "33",
		"Art. 6(1)(a)":        "6",
		"Art. 5 §2":           "5",
		"(5)":                 "5",
	} {
		if got := NormalizeNum(in); got != want {
			t.Errorf("NormalizeNum(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		lang, text string
		want       []string // article numbers in order
	}{
		{"en", "Preamble text\nArticle 1\nSubject matter\nArticle 2\nScope\nArticle 6(1) is a cross-reference at a line start\nArticle 3\nDefinitions", []string{"preamble", "1", "2", "3"}},
		{"zh", "第一条　为了保障网络安全。\n第二条　在境内建设网络。\n依照本法第二十一条的规定\n第三条　国家坚持。", []string{"1", "2", "3"}},
		{"ja", "第一条 目的\n第二条 定義\n第二条の二 追加\n第三条 責務", []string{"1", "2", "2-2", "3"}},
		{"ko", "제1조(목적)\n제2조(정의)\n제2조의2(추가)\n제3조", []string{"1", "2", "2-2", "3"}},
		{"es", "ARTÍCULO 1.- Objeto\nArtículo 2o.- Ámbito\nArtículo 3 bis.- Nuevo", []string{"1", "2", "3bis"}},
		{"pt", "Art. 1º Esta Lei dispõe\nArt. 2º A disciplina\nArt. 3º Esta Lei aplica-se", []string{"1", "2", "3"}},
		{"tr", "MADDE 1 – Amaç\nMADDE 2 – Kapsam", []string{"1", "2"}},
		{"ar", "المادة الأولى:\nالتعريفات\nالمادة الثانية:\nالنطاق\nالمادة الثالثة\nالأحكام", []string{"1", "2", "3"}},
		{"en", "No headings here at all.", []string{"all"}},
		{"en", "Article 1\nA\nArticle 2\nB\nANNEX I\nList of sectors", []string{"1", "2", "annex"}},
		{"zh", "第 1 條 目的\n第 1-1 條 補充\n第 1-2 條 再補充\n第 2 條 定義", nil}, // filled below with a custom split
		// a table of contents lists the headings before the body does: the body wins
		{"en", "Contents\nArticle 1\nArticle 2\nArticle 3\nArticle 1\nScope text\nArticle 2\nMore\nArticle 3\nEnd", []string{"preamble", "1", "2", "3"}},
	}
	for _, c := range cases {
		if c.want == nil {
			continue
		}
		var got []string
		for _, a := range Split(c.text, c.lang, "", 0) {
			got = append(got, a.Num)
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("Split(%s) = %v, want %v", c.lang, got, c.want)
		}
	}
	if got := Split("§ 314.1 Purpose\n§ 314.2 Definitions\nsee § 314.9 here\n§ 314.3 Standards", "en", `(?m)^§ (\d+\.\d+)`, 0); len(got) != 3 || got[2].Num != "314.3" {
		t.Errorf("CFR-style sections: %+v", got)
	}
	tw := Split("第 1 條 目的\n第 1-1 條 補充\n第 1-2 條 再補充\n第 2 條 定義", "zh", `(?m)^第 (\d+(?:-\d+)?) 條`, 0)
	if len(tw) != 4 || tw[2].Num != "1-2" {
		t.Errorf("sub-articles 1-1, 1-2 must each be kept: %+v", tw)
	}
	nca := Split("Contents\n1-1-1\n1-2-1\nControls\n1-1-1\nPolicies\n1-1-2\nReview\n1-2-1\nRoles:\n1-2-1-1 RACI\n1-2-1\n1-2-1-2 Job duties\n2-1-1-1 Inventory\n2-1-1\n2-1-1-2 Labels",
		"en", `(?m)^[ \t]*(\d-\d+-\d+(?:-\d+)?)\b`, 1000000)
	var ncaNums []string
	for _, a := range nca {
		ncaNums = append(ncaNums, a.Num)
	}
	if got := strings.Join(ncaNums, ","); got != "preamble,1-1-1,1-1-2,1-2-1,1-2-1-1,1-2-1-2,2-1-1-1,2-1-1-2" {
		t.Errorf("control ids (a control printed after its first subcontrol is dropped, not the subcontrol) = %s", got)
	}
	gappy := "Article 1\nA\nArticle 2\nB\nArticle 37\nC\nArticle 38\nD"
	if got := Split(gappy, "en", "", 0); len(got) != 3 || got[0].Num != "preamble" || got[1].Num != "37" {
		t.Errorf("default gap limit keeps one run (the rest becomes preamble), got %+v", got)
	}
	if got := Split(gappy, "en", "", 40); len(got) != 4 {
		t.Errorf("max_gap 40 should keep both runs of a partly enforced text, got %+v", got)
	}
	if got := Split("§ 1 A\n§ 2 B", "de", `(?m)^§ (\d+)`, 0); len(got) != 2 || got[1].Num != "2" {
		t.Errorf("custom split regex not used: %+v", got)
	}
}

func TestDecode(t *testing.T) {
	latin1 := []byte("Art. 1\xba Lei de prote\xe7\xe3o")
	if got, err := decode(latin1, "text/html; charset=ISO-8859-1"); err != nil || got != "Art. 1º Lei de proteção" {
		t.Errorf("latin-1 decode = %q, %v", got, err)
	}
	if _, err := decode([]byte{0x82, 0xa0, 0x82}, "text/html; charset=Shift_JIS"); err == nil {
		t.Error("Shift_JIS should be refused, not garbled")
	}
	if got := HTMLToText(`<html><head><title>x</title><script>evil()</script></head><body><p>Article&nbsp;1</p><p>Text <b>bold</b></p></body></html>`); got != "Article 1\nText bold" {
		t.Errorf("HTMLToText = %q", got)
	}
	// a self-closed script must not swallow the body up to a later closing tag
	if got := HTMLToText(`<script src="x.js"/><p>Article 1</p><p>Kept</p><noscript>n</noscript>`); got != "Article 1\nKept" {
		t.Errorf("HTMLToText with self-closed script = %q", got)
	}
	if _, err := decode([]byte{0xe7, 0xe3}, "text/html"); err == nil {
		t.Error("non-UTF-8 without a declared charset should be refused, not guessed")
	}
	if _, err := decode([]byte{0xe7, 0xe3}, "text/html; charset=utf-8"); err == nil || !strings.Contains(err.Error(), "not valid UTF-8") {
		t.Errorf("invalid bytes labelled UTF-8: %v", err)
	}
}

// corpus builds a small multilingual corpus in a temp dir via the real Sync path.
func corpus(t *testing.T) string {
	t.Helper()
	pages := map[string]string{
		"/zh":     "<p>第一条　为了保障网络安全，维护网络空间主权。</p><p>第四条　网络运营者应当在一小时内报告重大网络安全事件。</p>",
		"/tr":     "<p>MADDE 11 – Amaç</p><p>MADDE 12 – Veri sorumlusu, ihlal bildirimi yapar.</p>",
		"/ar":     "<p>المادة التاسعة عشرة:</p><p>تعريفات</p><p>المادة العشرون:</p><p>يجب إبلاغ الجهة المختصة عن تسرُّب البيانات الشخصية.</p>",
		"/en":     "<p>Article 32</p><p>Scope</p><p>Article 33</p><p>Notification of a personal data breach to the supervisory authority within 72 hours.</p>",
		"/en-old": "<p>Article 1</p><p>Scope of the old translation</p>",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := pages[r.URL.Path]; ok {
			fmt.Fprint(w, p)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	src := func(id, lang, path, kind, translates, date string, terms ...string) Source {
		return Source{ID: id, Title: id, Jurisdiction: "XX", Lang: lang, Kind: kind, Translates: translates,
			URL: srv.URL + path, Format: "html", VersionDate: date, Terms: terms}
	}
	sources := []Source{
		src("cn-test", "zh", "/zh", "official", "", "2026-01-01", "网络安全法", "shared-term"),
		src("tr-test", "tr", "/tr", "official", "", "2025-03-19", "KVKK", "two-officials"),
		src("sa-test", "ar", "/ar", "official", "", "2024-09-14"),
		src("eu-test", "en", "/en", "official", "", "2016-05-04", "GDPR", "two-officials"),
		src("cn-test-en", "en", "/en-old", "reference", "cn-test", "2017-06-01", "shared-term"),
	}
	path := filepath.Join(t.TempDir(), "laws.db")
	res, err := Sync(context.Background(), path, sources, nil, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Err != nil {
			t.Fatalf("sync %s: %v", r.ID, r.Err)
		}
	}
	return path
}

func TestCorpusLookupAndSearch(t *testing.T) {
	st, err := Open(corpus(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	for _, ref := range []string{"cn-test", "网络安全法", "kvkk", "GDPR"} {
		if _, err := st.Resolve(ref); err != nil {
			t.Errorf("Resolve(%q): %v", ref, err)
		}
	}
	if a, err := st.Article("cn-test", "第四条"); err != nil || !strings.Contains(a.Body, "一小时") {
		t.Errorf("Article(cn-test, 第四条) = %+v, %v", a, err)
	}
	if a, err := st.Article("sa-test", "20"); err != nil || !strings.Contains(a.Body, "البيانات") {
		t.Errorf("Article(sa-test, 20) = %+v, %v", a, err)
	}
	if _, err := st.Article("eu-test", "99"); err == nil {
		t.Error("missing article should error")
	}
	if a, err := st.Article("eu-test", "Art. 33(1)"); err != nil || a.Num != "33" {
		t.Errorf("a paragraph citation should find its article: %+v, %v", a, err)
	}
	if l, err := st.Resolve("SHARED-TERM"); err != nil || l.ID != "cn-test" {
		t.Errorf("a term shared with a translation should resolve to the official text: %+v, %v", l, err)
	}
	if _, err := st.Resolve("TWO-OFFICIALS"); err == nil || !strings.Contains(err.Error(), "eu-test, tr-test") {
		t.Errorf("a term shared by two official texts must be an error naming both: %v", err)
	}

	for _, c := range []struct{ q, law, wantLaw, wantNum string }{
		{"网络安全事件", "", "cn-test", "4"},            // trigram, CJK
		{"报告", "", "cn-test", "4"},                // 2 chars: LIKE fallback
		{"bildirimi", "", "tr-test", "12"},        // Turkish suffix
		{"البيانات الشخصية", "", "sa-test", "20"}, // Arabic, two words
		{"تَسَرُّب", "", "sa-test", "20"},         // vowel marks in the query, none in the text
		{"BREACH 72", "", "eu-test", "33"},        // case-insensitive, mixed lengths
		{"scope", "eu-test", "eu-test", "32"},     // filtered to one law
	} {
		hits, err := st.Search(c.q, c.law, 5)
		if err != nil || len(hits) == 0 || hits[0].LawID != c.wantLaw || hits[0].Num != c.wantNum {
			t.Errorf("Search(%q, %q) = %+v, %v; want %s art %s first", c.q, c.law, hits, err, c.wantLaw, c.wantNum)
		}
	}

	ref, _ := st.Resolve("cn-test-en")
	if w := st.Stale(ref); !strings.Contains(w, "2017-06-01") || !strings.Contains(w, "2026-01-01") {
		t.Errorf("stale translation not flagged: %q", w)
	}
	if off, _ := st.Resolve("cn-test"); st.Stale(off) != "" {
		t.Error("an official text is never stale against itself")
	}
}

func TestSyncKeepsOldVersionOnFailure(t *testing.T) {
	path := corpus(t)
	dead := []Source{{ID: "eu-test", Title: "x", Jurisdiction: "XX", Lang: "en", Kind: "official",
		URL: "https://127.0.0.1:1/gone", Format: "html", VersionDate: "2026-01-01"}}
	res, _ := Sync(context.Background(), path, dead, nil, http.DefaultClient)
	if res[0].Err == nil {
		t.Fatal("unreachable source should fail")
	}
	st, _ := Open(path)
	defer st.Close()
	if _, err := st.Article("eu-test", "33"); err != nil {
		t.Errorf("failed re-sync dropped the previous version: %v", err)
	}
}

func TestSyncRejectsPagesWithoutArticles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/home":
			fmt.Fprint(w, "<p>Welcome to the portal</p>")
		default:
			fmt.Fprint(w, "<p>Artículo 1. First law</p><p>Artículo 2. Again</p><p>LEY B</p><p>Artículo 1. Second law</p><p>Artículo 2. More</p>")
		}
	}))
	defer srv.Close()
	src := func(id, path, start string) Source {
		return Source{ID: id, Title: id, Jurisdiction: "XX", Lang: "es", Kind: "official", URL: srv.URL + path,
			Format: "html", VersionDate: "2025-01-01", Start: start}
	}
	path := filepath.Join(t.TempDir(), "laws.db")
	res, _ := Sync(context.Background(), path, []Source{src("xx-home", "/home", ""), src("xx-b", "/decree", "LEY B"), src("xx-c", "/decree", "LEY C")}, nil, srv.Client())
	if res[0].Err == nil {
		t.Error("a page with no article headings (e.g. a redirect to a home page) must fail")
	}
	if res[1].Err != nil || res[1].Articles != 3 { // preamble "LEY B" + 2 articles
		t.Errorf("start marker: %+v", res[1])
	}
	if res[2].Err == nil {
		t.Error("a missing start marker must fail")
	}
	st, _ := Open(path)
	defer st.Close()
	if a, _ := st.Article("xx-b", "1"); !strings.Contains(a.Body, "Second law") {
		t.Errorf("start marker should cut off the first law: %q", a.Body)
	}
}

func TestSyncUnknownID(t *testing.T) {
	_, err := Sync(context.Background(), filepath.Join(t.TempDir(), "x.db"), []Source{{ID: "eu-gdpr"}}, []string{"eu-gdrp"}, http.DefaultClient)
	if err == nil || !strings.Contains(err.Error(), "eu-gdrp") {
		t.Errorf("a mistyped id must be an error, got %v", err)
	}
}

func TestOpenMissing(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "none.db")); err != ErrNoCorpus {
		t.Errorf("Open(missing) = %v, want ErrNoCorpus", err)
	}
}

func TestManifest(t *testing.T) {
	if _, err := Manifest(); err != nil {
		t.Fatal(err)
	}
	bad := fstest.MapFS{"s/x.json": {Data: []byte(`[
	 {"id":"xx-a","title":"A","jurisdiction":"XX","lang":"en","kind":"official","url":"http://plain","format":"html","version_date":"2026-01-01"},
	 {"id":"xx-a","title":"B","jurisdiction":"XX","lang":"en","kind":"reference","translates":"xx-missing","url":"https://ok","format":"docx","version_date":"soon"},
	 {"id":"xx-b","title":"C","jurisdiction":"XX","lang":"en","kind":"reference","url":"https://ok","format":"pdf","version_date":"2026-01-01"}]`)}}
	_, err := LoadManifest(bad, "s")
	for _, want := range []string{"url must be https", "duplicate id", "format must be", "version_date", "translates must name", "xx-b: a reference text without translates needs a note"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("manifest validation missed %q: %v", want, err)
		}
	}
}

// TestSkillLawIDsExist: the skills' "Law texts" sections name corpus ids for
// the model to look up; a renamed or dropped source must not leave them dangling.
func TestSkillLawIDsExist(t *testing.T) {
	sources, err := Manifest()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, s := range sources {
		ids[s.ID] = true
	}
	files, _ := filepath.Glob("../../skills/*/*/SKILL.md")
	section := regexp.MustCompile(`(?s)## Law texts\n(.*?)(?:\n## |\z)`)
	ref := regexp.MustCompile("`([a-z]{2}-[a-z0-9-]+)(?: [^`]+)?`")
	n := 0
	for _, f := range files {
		raw, _ := os.ReadFile(f)
		m := section.FindSubmatch(raw)
		if m == nil {
			continue
		}
		for _, r := range ref.FindAllSubmatch(m[1], -1) {
			n++
			if !ids[string(r[1])] {
				t.Errorf("%s names law id %q, which is not in internal/laws/sources", f, r[1])
			}
		}
	}
	if n == 0 {
		t.Error("no skill names a law id; the check found nothing to check")
	}
}

// minimalPDF builds a one-page PDF whose text lines are lines, with a correct
// xref table, so the pdftotext path is tested without a binary fixture.
func minimalPDF(lines []string) []byte {
	var content strings.Builder
	content.WriteString("BT /F1 12 Tf 72 720 Td 16 TL\n")
	for _, l := range lines {
		fmt.Fprintf(&content, "(%s) Tj T*\n", l)
	}
	content.WriteString("ET")
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return []byte(b.String())
}

func TestSyncPDF(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed (the Docker image has it)")
	}
	config.C.AuditLog = filepath.Join(t.TempDir(), "audit.jsonl") // the runner audits the conversion
	pdf := minimalPDF([]string{"Article 1", "Scope of this act", "Article 2", "Report a breach within 72 hours"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(pdf)
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "laws.db")
	src := Source{ID: "xx-pdf", Title: "x", Jurisdiction: "XX", Lang: "en", Kind: "official", URL: srv.URL, Format: "pdf", VersionDate: "2025-01-01"}
	res, err := Sync(context.Background(), path, []Source{src}, nil, srv.Client())
	if err != nil || res[0].Err != nil || res[0].Articles != 2 {
		t.Fatalf("pdf sync: %+v, %v", res, err)
	}
	st, _ := Open(path)
	defer st.Close()
	if a, err := st.Article("xx-pdf", "2"); err != nil || !strings.Contains(a.Body, "72 hours") {
		t.Errorf("pdf article 2 = %+v, %v", a, err)
	}
	log, _ := os.ReadFile(config.C.AuditLog)
	if !strings.Contains(string(log), "laws_pdftotext") {
		t.Error("the pdftotext run must be audit-logged like any external binary")
	}
}

// TestPDFTextFolds: PDF text artefacts must not hide a match from search.
func TestPDFTextFolds(t *testing.T) {
	if norm("อำนาจ") != norm("อานาจ") {
		t.Error("Thai sara am must fold to sara aa (gazette PDFs drop the nikhahit)")
	}
	if got := ligatures.Replace("Notiﬁcación ﬂujo"); got != "Notificación flujo" {
		t.Errorf("ligatures = %q", got)
	}
}
