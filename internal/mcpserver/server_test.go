package mcpserver

import (
	"strings"
	"testing"

	"github.com/rhaist/marq/pi"
)

// TestUntrustedRuleInStep: Pi reads pi/SYSTEM.md and pi/SKILL.md, not the MCP
// instructions, so each must carry the same fence rule word for word.
func TestUntrustedRuleInStep(t *testing.T) {
	for name, text := range map[string]string{"pi/SYSTEM.md": pi.SystemPrompt, "pi/SKILL.md": pi.Skill} {
		if !strings.Contains(text, untrustedRule) {
			t.Errorf("%s does not carry untrustedRule verbatim; copy it from internal/mcpserver/server.go", name)
		}
	}
}
