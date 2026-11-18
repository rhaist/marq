package mcpserver

import (
	"strings"
	"testing"

	"github.com/rhaist/marq/internal/registry"
	"github.com/rhaist/marq/pi"
)

// TestUntrustedRuleInStep: Pi reads pi/SYSTEM.md and pi/SKILL.md, and a client
// may read marq://methodology alone, so each carries the fence rule word for word.
func TestUntrustedRuleInStep(t *testing.T) {
	for name, text := range map[string]string{"pi/SYSTEM.md": pi.SystemPrompt, "pi/SKILL.md": pi.Skill, "methodology.md": registry.Methodology} {
		if !strings.Contains(text, untrustedRule) {
			t.Errorf("%s does not carry untrustedRule verbatim; copy it from internal/mcpserver/server.go", name)
		}
	}
}
