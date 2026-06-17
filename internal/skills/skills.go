// Package skills is an on-demand knowledge library: per-vuln-class and
// per-technique playbooks as embedded markdown, tied to this server's actual
// tool names. The model loads a skill when it starts on that kind of work — the
// cheapest way to make a local model chain the tools competently (modeled on
// Strix's skills, minus the multi-agent machinery).
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

//go:embed library
var library embed.FS

// Meta is a skill's listing entry (from its YAML frontmatter).
type Meta struct {
	Name        string
	Category    string
	Description string
}

type skill struct {
	Meta
	Body string
}

var frontmatter = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)

// parse splits name/description frontmatter from the markdown body.
func parse(raw string) (name, desc, body string) {
	m := frontmatter.FindStringSubmatch(raw)
	if m == nil {
		return "", "", strings.TrimSpace(raw)
	}
	for _, line := range strings.Split(m[1], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "name":
			name = strings.TrimSpace(v)
		case "description":
			desc = strings.TrimSpace(v)
		}
	}
	return name, desc, strings.TrimSpace(raw[len(m[0]):])
}

// index is built once from the embedded library at startup.
var index = build()

func build() map[string]skill {
	out := map[string]skill{}
	_ = fs.WalkDir(library, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		raw, rerr := library.ReadFile(p)
		if rerr != nil {
			return nil
		}
		name, desc, body := parse(string(raw))
		if name == "" {
			name = strings.TrimSuffix(d.Name(), ".md")
		}
		out[name] = skill{Meta: Meta{Name: name, Category: path.Base(path.Dir(p)), Description: desc}, Body: body}
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
	for _, n := range strings.Split(names, ",") {
		if n = strings.TrimSpace(n); n != "" {
			wanted = append(wanted, n)
		}
	}
	if len(wanted) == 0 {
		return "error: name a skill to load. Available:\n" + IndexText()
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
