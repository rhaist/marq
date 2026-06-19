// Package config holds all server settings, read once from the environment at
// startup. It mirrors the env-driven knobs of the original Python server
// (MARQ_*); adding a knob means adding a field plus an env read here.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the resolved configuration for a server/agent run.
type Config struct {
	// AuditLog is where the structured audit log is written (JSON lines).
	AuditLog string
	// CommandTimeout is the per-command wall-clock timeout in seconds.
	CommandTimeout int
	// MaxCommandTimeout is the hard ceiling a tool may request via a per-call
	// timeout override.
	MaxCommandTimeout int
	// MaxOutputChars caps characters of tool output returned to the model.
	MaxOutputChars int
	// Operator / Engagement / ScopeNote are recorded in every audit record and
	// surfaced in the authorization banner.
	Operator   string
	Engagement string
	ScopeNote  string
	// AllowRawShell exposes the generic run_shell escape hatch when true.
	AllowRawShell bool
	// WorkDir is the engagement working area (findings, job dirs live here).
	WorkDir string

	// Model runtime — used by the TUI agent host; ignored in MCP serve mode
	// (there the external client brings its own model).
	ModelBaseURL string
	ModelName    string
	ModelAPIKey  string

	// TUIConfigPath is where the TUI setup form persists its choices so the
	// next launch skips setup. Defaults to <WorkDir>/.marq/tui.json (the only
	// host-mounted path in the container); override with MARQ_TUI_CONFIG.
	TUIConfigPath string
}

func env(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return def
}

func envInt(name string, def int) int {
	if v, ok := os.LookupEnv(name); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func envBool(name string, def bool) bool {
	v, ok := os.LookupEnv(name)
	if !ok {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Load reads configuration from the environment. Precedence for the
// TUI-editable fields (operator/engagement/scope/model_*): explicit env var >
// persisted TUI config file > built-in default. Non-editable knobs stay
// env-only.
func Load() Config {
	c := Config{
		AuditLog:          env("MARQ_AUDIT_LOG", "/var/log/marq/audit.jsonl"),
		CommandTimeout:    envInt("MARQ_TIMEOUT", 900),
		MaxCommandTimeout: envInt("MARQ_MAX_TIMEOUT", 3600),
		MaxOutputChars:    envInt("MARQ_MAX_OUTPUT", 60000),
		AllowRawShell:     envBool("MARQ_ALLOW_RAW_SHELL", true),
		WorkDir:           env("MARQ_WORK_DIR", "/work"),
		ModelBaseURL:      env("MARQ_MODEL_URL", "http://host.docker.internal:11434/v1"),
		ModelName:         env("MARQ_MODEL", "huihui_ai/Qwen3.6-abliterated:27b"),
		ModelAPIKey:       env("MARQ_MODEL_KEY", "ollama"),
		Operator:          env("MARQ_OPERATOR", "unknown"),
		Engagement:        env("MARQ_ENGAGEMENT", "unspecified"),
		ScopeNote:         env("MARQ_SCOPE", ""),
	}
	c.TUIConfigPath = env("MARQ_TUI_CONFIG", filepath.Join(c.WorkDir, ".marq", "tui.json"))

	// Overlay the persisted TUI file for fields not explicitly set by env.
	// os.LookupEnv distinguishes "unset" from "set to empty".
	if fc := loadTUIFile(c.TUIConfigPath); fc != nil {
		if _, set := os.LookupEnv("MARQ_OPERATOR"); !set && fc.Operator != nil {
			c.Operator = *fc.Operator
		}
		if _, set := os.LookupEnv("MARQ_ENGAGEMENT"); !set && fc.Engagement != nil {
			c.Engagement = *fc.Engagement
		}
		if _, set := os.LookupEnv("MARQ_SCOPE"); !set && fc.ScopeNote != nil {
			c.ScopeNote = *fc.ScopeNote
		}
		if _, set := os.LookupEnv("MARQ_MODEL_URL"); !set && fc.ModelBaseURL != nil {
			c.ModelBaseURL = *fc.ModelBaseURL
		}
		if _, set := os.LookupEnv("MARQ_MODEL"); !set && fc.ModelName != nil {
			c.ModelName = *fc.ModelName
		}
		if _, set := os.LookupEnv("MARQ_MODEL_KEY"); !set && fc.ModelAPIKey != nil {
			c.ModelAPIKey = *fc.ModelAPIKey
		}
	}
	return c
}

// tuiFile is the on-disk shape of the TUI setup form. Pointer fields so an
// omitted key is distinguishable from an explicitly-empty value.
type tuiFile struct {
	Operator     *string `json:"operator,omitempty"`
	Engagement   *string `json:"engagement,omitempty"`
	ScopeNote    *string `json:"scope_note,omitempty"`
	ModelBaseURL *string `json:"model_base_url,omitempty"`
	ModelName    *string `json:"model_name,omitempty"`
	ModelAPIKey  *string `json:"model_api_key,omitempty"`
}

// loadTUIFile reads the persisted TUI config. Returns nil if the file is
// missing or unreadable (a malformed file is ignored, not fatal — the user
// just re-runs setup).
func loadTUIFile(path string) *tuiFile {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f tuiFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil
	}
	return &f
}

// TUIConfigExists reports whether a persisted TUI config is present — used by
// the TUI to decide whether to skip the setup screen on launch.
func TUIConfigExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

// SaveTUIConfig persists the TUI-editable fields of c to c.TUIConfigPath
// (creating parent dirs as needed). Best-effort: callers treat an error as
// non-fatal (next launch simply re-runs setup).
func SaveTUIConfig(c Config) error {
	if c.TUIConfigPath == "" {
		return fmt.Errorf("no TUI config path configured")
	}
	f := tuiFile{
		Operator:     &c.Operator,
		Engagement:   &c.Engagement,
		ScopeNote:    &c.ScopeNote,
		ModelBaseURL: &c.ModelBaseURL,
		ModelName:    &c.ModelName,
		ModelAPIKey:  &c.ModelAPIKey,
	}
	if err := os.MkdirAll(filepath.Dir(c.TUIConfigPath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.TUIConfigPath, b, 0o600)
}

// C is the process-wide configuration, resolved once at startup (mirrors the
// Python module-level CONFIG). Components read config.C directly.
var C = Load()

// Banner returns the authorization notice shown to the operator before tools run.
func (c Config) Banner() string {
	scope := c.ScopeNote
	if scope == "" {
		scope = "(none provided — set MARQ_SCOPE)"
	}
	return fmt.Sprintf(
		"AUTHORIZED USE ONLY. This server runs active security testing tools. "+
			"Only use it against systems you own or are explicitly authorized in "+
			"writing to test. All invocations are audit-logged.\n"+
			"  operator   : %s\n"+
			"  engagement : %s\n"+
			"  scope note : %s\n"+
			"  audit log  : %s",
		c.Operator, c.Engagement, scope, c.AuditLog,
	)
}
