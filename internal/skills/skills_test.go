package skills

import (
	"io/fs"
	"strings"
	"testing"
)

// TestNoDuplicateSkillNames guards build(): the index is keyed by frontmatter
// name, so two skills sharing a name would silently overwrite each other and
// vanish with no error. Assert the loaded count equals the embedded-file count.
func TestNoDuplicateSkillNames(t *testing.T) {
	var files int
	_ = fs.WalkDir(library, ".", func(p string, d fs.DirEntry, _ error) error {
		if d != nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
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
	out := LoadMany("sqli, xss")
	if !strings.Contains(out, "skill: sqli") || !strings.Contains(out, "skill: xss") {
		t.Errorf("LoadMany did not return both skills: %q", out)
	}
	if got := LoadMany("a,b,c,d,e,f"); !strings.Contains(got, "at most 5") {
		t.Errorf("LoadMany did not cap at 5: %q", got)
	}
}
