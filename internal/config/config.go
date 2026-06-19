// Package config holds all server settings, read once from the environment at
// startup. It mirrors the env-driven knobs of the original Python server
// (MARQ_*); adding a knob means adding a field plus an env read here.
package config

import (
	"fmt"
	"os"
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

// Load reads configuration from the environment.
func Load() Config {
	return Config{
		AuditLog:          env("MARQ_AUDIT_LOG", "/var/log/marq/audit.jsonl"),
		CommandTimeout:    envInt("MARQ_TIMEOUT", 900),
		MaxCommandTimeout: envInt("MARQ_MAX_TIMEOUT", 3600),
		MaxOutputChars:    envInt("MARQ_MAX_OUTPUT", 60000),
		Operator:          env("MARQ_OPERATOR", "unknown"),
		Engagement:        env("MARQ_ENGAGEMENT", "unspecified"),
		ScopeNote:         env("MARQ_SCOPE", ""),
		AllowRawShell:     envBool("MARQ_ALLOW_RAW_SHELL", true),
		WorkDir:           env("MARQ_WORK_DIR", "/work"),
		ModelBaseURL:      env("MARQ_MODEL_URL", "http://host.docker.internal:11434/v1"),
		ModelName:         env("MARQ_MODEL", "huihui_ai/Qwen3.6-abliterated:27b"),
		ModelAPIKey:       env("MARQ_MODEL_KEY", "ollama"),
	}
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
