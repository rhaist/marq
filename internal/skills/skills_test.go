package skills

import (
	"strings"
	"testing"
)

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
