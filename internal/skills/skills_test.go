package skills

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestNoDuplicateSkillNames guards build(): the index is keyed by frontmatter
// name, so two skills sharing a name would silently overwrite each other and
// vanish with no error. Assert the loaded count equals the embedded-file count.
func TestNoDuplicateSkillNames(t *testing.T) {
	var files int
	_ = fs.WalkDir(library, ".", func(p string, d fs.DirEntry, _ error) error {
		if d != nil && !d.IsDir() && d.Name() == "SKILL.md" {
			files++
		}
		return nil
	})
	if got := len(List()); got != files {
		t.Fatalf("loaded %d skills but %d .md files embedded — a duplicate name: overwrote one", got, files)
	}
}

func TestLibraryLoads(t *testing.T) {
	list := List()
	if len(list) == 0 {
		t.Fatal("no skills embedded")
	}
	for _, m := range list {
		if m.Name == "" || m.Description == "" {
			t.Errorf("skill missing name/description: %+v", m)
		}
		body, err := Load(m.Name)
		if err != nil {
			t.Errorf("Load(%q) failed: %v", m.Name, err)
		}
		if strings.TrimSpace(body) == "" {
			t.Errorf("skill %q has empty body", m.Name)
		}
		if strings.HasPrefix(body, "---") {
			t.Errorf("skill %q frontmatter not stripped", m.Name)
		}
	}
}

func TestLoadMany(t *testing.T) {
	out := LoadMany("recon-footprint, ot-ics")
	if !strings.Contains(out, "skill: recon-footprint") || !strings.Contains(out, "skill: ot-ics") {
		t.Errorf("LoadMany did not return both skills: %q", out)
	}
	if got := LoadMany("a,b,c,d,e,f"); !strings.Contains(got, "at most 5") {
		t.Errorf("LoadMany did not cap at 5: %q", got)
	}
}

// TestAgentSkillsSpec keeps the pack installable by other agents: the
// agentskills.io rules for name/description, name == directory, and a parseable
// review date that isn't in the future.
func TestAgentSkillsSpec(t *testing.T) {
	nameRe := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	descRe := regexp.MustCompile(`(?m)^description: (.*)$`)
	// the exact form /refresh-skills' sed rewrites; anything else never gets bumped
	verifiedRe := regexp.MustCompile(`(?m)^  verified: "\d{4}-\d{2}-\d{2}"$`)
	_ = fs.WalkDir(library, ".", func(p string, d fs.DirEntry, _ error) error {
		if d == nil || d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		raw, _ := library.ReadFile(p)
		name, desc, verified, _ := parse(string(raw))
		if dir := path.Base(path.Dir(p)); name != dir {
			t.Errorf("%s: name %q must match its directory %q", p, name, dir)
		}
		if len(name) > 64 || !nameRe.MatchString(name) {
			t.Errorf("%s: invalid name %q", p, name)
		}
		// The Go parser is line-based; strict YAML (npx skills) rejects an
		// unquoted value containing ": ", so such descriptions must be quoted.
		if line := descRe.FindStringSubmatch(string(raw)); line != nil &&
			!strings.HasPrefix(line[1], `"`) && strings.Contains(line[1], ": ") {
			t.Errorf("%s: description contains \": \" and must be double-quoted for strict YAML", p)
		}
		// Agents pick a skill from its description alone (Anthropic skill-authoring
		// best practices): say what it covers, then when to use it.
		if !strings.Contains(desc, "Use when") {
			t.Errorf("%s: description needs a \"Use when ...\" trigger clause", p)
		}
		if desc == "" || len(desc) > 1024 {
			t.Errorf("%s: description must be 1-1024 chars (got %d)", p, len(desc))
		}
		// a day of slack: dates are written in local time, parsed as UTC midnight
		if v, err := time.Parse(time.DateOnly, verified); err != nil || v.After(time.Now().Add(24*time.Hour)) || !verifiedRe.MatchString(string(raw)) {
			t.Errorf("%s: metadata.verified %q must be `  verified: \"YYYY-MM-DD\"`, not in the future", p, verified)
		}
		return nil
	})
}

func TestStale(t *testing.T) {
	newest := time.Time{}
	for _, m := range List() {
		if v, _ := time.Parse(time.DateOnly, m.Verified); v.After(newest) {
			newest = v
		}
	}
	for _, m := range Stale(newest, 0) {
		if m.Verified == newest.Format(time.DateOnly) {
			t.Errorf("a skill verified exactly at now must not be stale: %+v", m)
		}
	}
	if got := Stale(newest.AddDate(1, 0, 0), 8*7*24*time.Hour); len(got) != len(List()) {
		t.Errorf("a year later every skill should be stale, got %d of %d", len(got), len(List()))
	}
}

// TestPluginListsEveryCategory: Claude Code plugins scan one level, so
// plugin.json must name every skills/<category>/ or that category never loads.
func TestPluginListsEveryCategory(t *testing.T) {
	raw, err := os.ReadFile("../../skills/.claude-plugin/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct{ Skills []string }
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, p := range manifest.Skills {
		listed[strings.Trim(strings.TrimPrefix(p, "./"), "/")] = true
	}
	dirs, _ := fs.ReadDir(library, "skills")
	for _, d := range dirs {
		if d.IsDir() && !listed[d.Name()] {
			t.Errorf("skills/%s/ is missing from plugin.json \"skills\"", d.Name())
		}
		delete(listed, d.Name())
	}
	for p := range listed {
		t.Errorf("plugin.json lists %q, which is not a skills/ category", p)
	}
}
