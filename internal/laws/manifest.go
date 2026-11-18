package laws

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Source is one law (or one government-published translation of it) that
// `marq laws sync` fetches from its official site. Sources live in
// sources/<region>.json; the corpus itself is never committed.
type Source struct {
	ID           string   `json:"id"`           // stable lookup id, e.g. "eu-gdpr", "cn-incident-measures"
	Title        string   `json:"title"`        // English title
	NativeTitle  string   `json:"native_title"` // official title in the original language
	Jurisdiction string   `json:"jurisdiction"` // ISO-3166 alpha-2 or "EU"
	Lang         string   `json:"lang"`         // language of THIS text (selects the heading pattern)
	Kind         string   `json:"kind"`         // "official" (binding text) or "reference" (government translation)
	Translates   string   `json:"translates"`   // for a reference translation: id of the official source
	URL          string   `json:"url"`          // official https URL
	Format       string   `json:"format"`       // "html" (also XML), "text" or "pdf" (converted with pdftotext)
	VersionDate  string   `json:"version_date"` // date of the consolidated version this URL serves (YYYY-MM-DD)
	Terms        []string `json:"terms"`        // abbreviations and key terms in any language, e.g. "PIPL", "个人信息保护法"
	Split        string   `json:"split"`        // optional heading regex overriding the language default
	Note         string   `json:"note"`         // optional caveat shown with every lookup
	Start        string   `json:"start"`        // optional: drop the page text before the first occurrence of this string (a law inside a multi-law decree)
	MaxGap       int      `json:"max_gap"`      // optional: largest numbering jump between headings (default 10), for partly enforced texts
}

//go:embed sources/*.json
var sourcesFS embed.FS

// Manifest returns every embedded source, validated.
func Manifest() ([]Source, error) { return LoadManifest(sourcesFS, "sources") }

var idRE = regexp.MustCompile(`^[a-z]{2}(-[a-z0-9]+)+$`)

// LoadManifest reads and validates all *.json source lists in dir.
func LoadManifest(fsys fs.FS, dir string) ([]Source, error) {
	files, err := fs.Glob(fsys, dir+"/*.json")
	if err != nil {
		return nil, err
	}
	var all []Source
	for _, f := range files {
		raw, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var list []Source
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		all = append(all, list...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return all, validate(all)
}

func validate(all []Source) error {
	byID := map[string]Source{}
	var errs []string
	bad := func(s Source, format string, a ...any) {
		errs = append(errs, s.ID+": "+fmt.Sprintf(format, a...))
	}
	for _, s := range all {
		if _, dup := byID[s.ID]; dup {
			bad(s, "duplicate id")
		}
		byID[s.ID] = s
		if !idRE.MatchString(s.ID) {
			bad(s, "id must look like jurisdiction-name (lowercase, e.g. eu-gdpr)")
		}
		if u, err := url.Parse(s.URL); err != nil || u.Scheme != "https" || u.Host == "" {
			bad(s, "url must be https")
		}
		if s.Title == "" || s.Jurisdiction == "" || s.Lang == "" {
			bad(s, "title, jurisdiction and lang are required")
		}
		if s.Format != "html" && s.Format != "text" && s.Format != "pdf" {
			bad(s, "format must be html, text or pdf")
		}
		if _, err := time.Parse(time.DateOnly, s.VersionDate); err != nil {
			bad(s, "version_date must be YYYY-MM-DD")
		}
		if s.Split != "" {
			if _, err := regexp.Compile(s.Split); err != nil {
				bad(s, "split: %v", err)
			}
		} else if headingRE[s.Lang] == "" {
			bad(s, "no heading pattern for lang %q; add a split regex", s.Lang)
		}
		switch s.Kind {
		case "official":
			if s.Translates != "" {
				bad(s, "an official text translates nothing")
			}
		case "reference":
		default:
			bad(s, "kind must be official or reference")
		}
	}
	for _, s := range all {
		if s.Kind == "reference" {
			if o, ok := byID[s.Translates]; !ok || o.Kind != "official" {
				bad(s, "translates must name an official source")
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("law sources: %s", strings.Join(errs, "; "))
	}
	return nil
}
