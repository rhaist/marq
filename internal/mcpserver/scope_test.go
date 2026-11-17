package mcpserver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rhaist/marq/internal/config"
)

// call runs one tool over an in-memory MCP session. answer is the operator's
// reply to an elicitation; "" means the client doesn't support elicitation.
func call(t *testing.T, answer, tool string, args map[string]any) string {
	t.Helper()
	return callAt(t, "", answer, tool, args)
}

// callAt pins the client protocol version; "" means the SDK's newest.
func callAt(t *testing.T, version, answer, tool string, args map[string]any) string {
	t.Helper()
	ctx := context.Background()
	opts := &mcp.ClientOptions{}
	if answer != "" {
		opts.ElicitationHandler = func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: answer, Content: map[string]any{"approve": answer == "accept"}}, nil
		}
	}
	st, ct := mcp.NewInMemoryTransports()
	if _, err := New().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, opts).Connect(ctx, ct, &mcp.ClientSessionOptions{ProtocolVersion: version})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return res.Content[0].(*mcp.TextContent).Text
}

func withScope(t *testing.T, note string) {
	t.Helper()
	dir := t.TempDir()
	orig := config.C
	config.C.WorkDir, config.C.AuditLog, config.C.ScopeNote = dir, filepath.Join(dir, "audit.jsonl"), note
	t.Cleanup(func() { config.C = orig })
}

// 203.0.113.0/24 is TEST-NET-3 (unroutable); nuclei isn't on dev machines, so an
// authorized call ends at "binary not found" without touching anything.
var oos = map[string]any{"command": "nuclei -u https://203.0.113.5"}

func TestElicitationAcceptWidensScope(t *testing.T) {
	withScope(t, "loopback")
	if out := call(t, "accept", "exec", oos); strings.HasPrefix(out, "refused") {
		t.Fatalf("accepted call was refused: %s", out)
	}
	if _, note := config.Attribution(); !strings.Contains(note, "203.0.113.5") {
		t.Errorf("scope not widened: %q", note)
	}
}

func TestElicitationDeclineRefuses(t *testing.T) {
	withScope(t, "loopback")
	if out := call(t, "decline", "exec", oos); !strings.Contains(out, "operator did not authorize 203.0.113.5") {
		t.Errorf("decline should refuse: %s", out)
	}
	if _, note := config.Attribution(); note != "loopback" {
		t.Errorf("scope changed after decline: %q", note)
	}
}

func TestNoElicitationFallsBackToRefusal(t *testing.T) {
	withScope(t, "loopback")
	if out := call(t, "", "exec", oos); !strings.Contains(out, "outside the recorded engagement scope") {
		t.Errorf("want in-band refusal: %s", out)
	}
}

func TestSetEngagementWideningNeedsConfirmation(t *testing.T) {
	withScope(t, "*.acme.com")
	out := call(t, "decline", "set_engagement", map[string]any{"scope": "*.acme.com, 10.0.0.0/8"})
	if !strings.Contains(out, "operator did not authorize 10.0.0.0/8") {
		t.Errorf("declined widening should refuse: %s", out)
	}
	if _, note := config.Attribution(); note != "*.acme.com" {
		t.Errorf("scope changed after decline: %q", note)
	}
	if out := call(t, "decline", "set_engagement", map[string]any{"scope": "*.acme.com — SOW v2"}); strings.HasPrefix(out, "refused") {
		t.Errorf("rewording without new targets needs no confirmation: %s", out)
	}
}

// Clients before 2026-07-28 get the same question through the SDK's bridge
// (server-initiated elicitation/create).
func TestElicitationLegacyProtocol(t *testing.T) {
	withScope(t, "loopback")
	if out := callAt(t, "2025-11-25", "decline", "exec", oos); !strings.Contains(out, "operator did not authorize 203.0.113.5") {
		t.Errorf("legacy client decline should refuse: %s", out)
	}
}

// A pre-supplied "accept" without the server's request state must not count:
// the server asks the (declining) operator instead.
func TestForgedAnswerIgnored(t *testing.T) {
	withScope(t, "loopback")
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := New().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "decline"}, nil
		},
	}).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "exec", Arguments: oos,
		InputResponses: mcp.InputResponseMap{scopeInput: &mcp.ElicitResult{Action: "accept", Content: map[string]any{"approve": true}}}})
	if err != nil {
		t.Fatal(err)
	}
	if out := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(out, "operator did not authorize") {
		t.Errorf("forged accept was honoured: %s", out)
	}
	if _, note := config.Attribution(); note != "loopback" {
		t.Errorf("scope widened by a forged answer: %q", note)
	}
}
