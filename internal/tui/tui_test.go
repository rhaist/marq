package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"marq/internal/config"
)

// TestSetupViewRendering checks the setup screen renders and starts on the
// LM Studio preset (the default) when no persisted config exists.
func TestSetupViewRendering(t *testing.T) {
	withTempTUIConfig(t)
	m := newModel()
	if m.view != viewSetup {
		t.Fatalf("expected setup view, got %v", m.view)
	}
	out := m.setupView()
	if !strings.Contains(out, "marq · setup") || !strings.Contains(out, "Backend") {
		t.Fatalf("setup view missing expected content:\n%s", out)
	}
	if !strings.Contains(out, "[ LM Studio ]") {
		t.Fatalf("expected LM Studio preset selected:\n%s", out)
	}
}

// TestSetupBackendCycle verifies cycling the backend selector applies the
// next preset (Ollama) to Base URL + API key.
func TestSetupBackendCycle(t *testing.T) {
	withTempTUIConfig(t)
	m := newModel()
	m.w, m.h = 80, 24
	m.focus = fiBackend

	m.handleSetupKey(tea.KeyMsg{Type: tea.KeyRight})
	if m.fields[fiBackend].idx != 1 {
		t.Fatalf("backend idx = %d, want 1 (Ollama)", m.fields[fiBackend].idx)
	}
	if got := m.fields[fiBaseURL].input.Value(); got != backends[1].baseURL {
		t.Fatalf("base URL preset = %q, want %q", got, backends[1].baseURL)
	}
	if got := m.fields[fiKey].input.Value(); got != backends[1].apiKey {
		t.Fatalf("api key preset = %q, want %q", got, backends[1].apiKey)
	}

	// Left cycles back to LM Studio and restores its preset.
	m.handleSetupKey(tea.KeyMsg{Type: tea.KeyLeft})
	if m.fields[fiBackend].idx != 0 {
		t.Fatalf("backend idx = %d, want 0", m.fields[fiBackend].idx)
	}
	if got := m.fields[fiBaseURL].input.Value(); got != backends[0].baseURL {
		t.Fatalf("base URL preset = %q, want %q", got, backends[0].baseURL)
	}
}

// TestSetupProceedCommits verifies Ctrl+S (save without check) writes config,
// mutates config.C, and switches to the chat view.
func TestSetupProceedCommits(t *testing.T) {
	withTempTUIConfig(t)
	m := newModel()
	m.w, m.h = 80, 24
	m.focus = fiBackend

	// Switch to the next backend (Ollama) and edit a couple of fields.
	m.handleSetupKey(tea.KeyMsg{Type: tea.KeyRight})
	m.fields[fiOperator].input.SetValue("alice")
	m.fields[fiScope].input.SetValue("*.example.com")

	m.handleSetupKey(tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.view != viewChat {
		t.Fatalf("expected chat view after proceed, got %v", m.view)
	}
	if config.C.ModelBaseURL != backends[1].baseURL {
		t.Fatalf("config.C.ModelBaseURL = %q, want %q", config.C.ModelBaseURL, backends[1].baseURL)
	}
	if config.C.Operator != "alice" || config.C.ScopeNote != "*.example.com" {
		t.Fatalf("config.C not updated: operator=%q scope=%q", config.C.Operator, config.C.ScopeNote)
	}
	if !config.TUIConfigExists(config.C.TUIConfigPath) {
		t.Fatal("config file should have been written by proceed")
	}
}

// TestChatReopenSetup verifies Ctrl+S from the chat view re-opens setup
// preloaded with the current config.
func TestChatReopenSetup(t *testing.T) {
	withTempTUIConfig(t)
	m := newModel()
	m.w, m.h = 80, 24
	m.view = viewChat

	m.handleChatKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.view != viewSetup {
		t.Fatalf("expected setup view after Ctrl+S, got %v", m.view)
	}
	if m.setupState != setupEditing {
		t.Fatalf("expected setupEditing state, got %v", m.setupState)
	}
}

// TestStartCheckEmptyURL verifies that starting a check with an empty Base URL
// short-circuits to a failed checked state instead of probing.
func TestStartCheckEmptyURL(t *testing.T) {
	withTempTUIConfig(t)
	m := newModel()
	m.fields[fiBaseURL].input.SetValue("")
	cmd := m.startCheck()
	if cmd != nil {
		t.Fatalf("expected nil cmd on empty URL, got non-nil")
	}
	if m.setupState != setupChecked || m.connOK {
		t.Fatalf("expected setupChecked/connOK=false, got state=%v ok=%v", m.setupState, m.connOK)
	}
}

// withTempTUIConfig points config.C at a fresh nonexistent path so newModel
// starts in the setup view, and so proceed() writes to a writable location.
func withTempTUIConfig(t *testing.T) {
	t.Helper()
	config.C.TUIConfigPath = filepath.Join(t.TempDir(), "tui.json")
}
