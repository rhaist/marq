// Package skills is an on-demand knowledge library: expert playbooks as
// embedded markdown spanning offensive, malware, threat-intel, and governance
// (GRC, standards, CISO) domains, tied to this server's actual tool names where
// relevant. The model loads a skill when it starts on that kind of work — the
// cheapest way to make a local model chain the tools competently (modeled on
// Strix's skills, minus the multi-agent machinery).
package skills

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rhaist/marq"
)

// library is the Agent Skills pack: skills/<category>/<name>/SKILL.md.
var library = marq.Skills

// Meta is a skill's listing entry (from its YAML frontmatter).
type Meta struct {
	Name        string
	Category    string
	Description string
	Verified    string // metadata.verified: date of the last review sweep (YYYY-MM-DD)
}

type skill struct {
	Meta
	Body string
}

var frontmatter = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)

// parse splits name/description/verified frontmatter from the markdown body.
// Line-based on purpose: `verified` sits under `metadata:`, and trimming the
// indent is all the YAML this flat frontmatter needs.
func parse(raw string) (name, desc, verified, body string) {
	m := frontmatter.FindStringSubmatch(raw)
	if m == nil {
		return "", "", "", strings.TrimSpace(raw)
	}
	for line := range strings.SplitSeq(m[1], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "name":
			name = strings.Trim(strings.TrimSpace(v), `"'`)
		case "description":
			desc = strings.Trim(strings.TrimSpace(v), `"'`)
		case "verified":
			verified = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return name, desc, verified, strings.TrimSpace(raw[len(m[0]):])
}

// index is built once from the embedded library at startup.
var index = build()

func build() map[string]skill {
	out := map[string]skill{}
	_ = fs.WalkDir(library, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		raw, rerr := library.ReadFile(p)
		if rerr != nil {
			return nil
		}
		dir := path.Dir(p) // skills/<category>/<name>
		name, desc, verified, body := parse(string(raw))
		if name == "" {
			name = path.Base(dir)
		}
		out[name] = skill{Meta: Meta{Name: name, Category: path.Base(path.Dir(dir)), Description: desc, Verified: verified}, Body: body}
		return nil
	})
	return out
}

// List returns every skill's metadata, sorted by category then name.
func List() []Meta {
	metas := make([]Meta, 0, len(index))
	for _, s := range index {
		metas = append(metas, s.Meta)
	}
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].Category != metas[j].Category {
			return metas[i].Category < metas[j].Category
		}
		return metas[i].Name < metas[j].Name
	})
	return metas
}

// IndexText is a "- name (category): description" list for tool docs / prompts.
func IndexText() string {
	var b strings.Builder
	for _, m := range List() {
		fmt.Fprintf(&b, "- %s (%s): %s\n", m.Name, m.Category, m.Description)
	}
	return b.String()
}

// Load returns one skill's markdown body.
func Load(name string) (string, error) {
	if s, ok := index[strings.TrimSpace(name)]; ok {
		return s.Body, nil
	}
	return "", fmt.Errorf("unknown skill %q", name)
}

// LoadMany loads up to 5 comma-separated skills, concatenated with headers.
// Unknown names are reported inline rather than failing the whole call.
func LoadMany(names string) string {
	var wanted []string
	for n := range strings.SplitSeq(names, ",") {
		if n = strings.TrimSpace(n); n != "" {
			wanted = append(wanted, n)
		}
	}
	if len(wanted) == 0 {
		return "Available skills (load one or more by name):\n" + IndexText()
	}
	if len(wanted) > 5 {
		return fmt.Sprintf("error: at most 5 skills per call (got %d)", len(wanted))
	}
	var b strings.Builder
	for _, n := range wanted {
		body, err := Load(n)
		if err != nil {
			fmt.Fprintf(&b, "## %s\n%s — available skills:\n%s\n", n, err.Error(), IndexText())
			continue
		}
		fmt.Fprintf(&b, "## skill: %s\n\n%s\n\n", n, body)
	}
	return strings.TrimSpace(b.String())
}

// Stale returns the skills whose last review sweep is older than maxAge at now,
// or whose verified date is missing or unparsable — the refresh to-do list.
func Stale(now time.Time, maxAge time.Duration) []Meta {
	var out []Meta
	for _, m := range List() {
		d, err := time.Parse(time.DateOnly, m.Verified)
		if err != nil || now.Sub(d) > maxAge {
			out = append(out, m)
		}
	}
	return out
}
