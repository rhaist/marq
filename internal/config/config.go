// Package config holds all server settings, read once from the environment at
// startup (MARQ_*); adding a knob means adding a field plus an env read here.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// engMu guards the only mutable field of C — Engagement — which
// set_engagement can write while, in `serve` mode, the go-sdk dispatches other
// tool handlers concurrently (their audit records read the same fields). Every
// other field is set once at Load() and read-only thereafter.
var engMu sync.RWMutex

// Config is the resolved configuration for a server/agent run.
type Config struct {
	// AuditLog is where the structured audit log is written (JSON lines).
	AuditLog string
	// CommandTimeout is the per-command wall-clock timeout in seconds.
	CommandTimeout int
	// MaxCommandTimeout caps how long a background job may run.
	MaxCommandTimeout int
	// MaxOutputChars caps characters of tool output returned to the model.
	MaxOutputChars int
	// Operator / Engagement (label + purpose) are recorded in every audit record
	// and surfaced in the banner.
	Operator   string
	Engagement string
	// SkillsOnly serves only the in-process knowledge tools (skills, findings,
	// files, info) and drops the exec and job tools, so the server runs
	// standalone without the tool image — the "cyber skills for your LLM" mode.
	SkillsOnly bool
	// WorkDir is the engagement working area (findings, job dirs live here).
	WorkDir string
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

// Load reads configuration from the environment (MARQ_*). Operator is the
// human/env-set audit attribution anchor; engagement defaults from env
// but can be set at runtime via SetEngagement (the set_engagement tool), which
// persists them under WorkDir so they survive across `marq run` processes that
// each re-Load.
func Load() Config {
	cfg := Config{
		AuditLog:          env("MARQ_AUDIT_LOG", "/var/log/marq/audit.jsonl"),
		CommandTimeout:    envInt("MARQ_TIMEOUT", 900),
		MaxCommandTimeout: envInt("MARQ_MAX_TIMEOUT", 3600),
		MaxOutputChars:    envInt("MARQ_MAX_OUTPUT", 60000),
		SkillsOnly:        envBool("MARQ_SKILLS_ONLY", false),
		WorkDir:           env("MARQ_WORK_DIR", "/work"),
		Operator:          env("MARQ_OPERATOR", "marq"),
		Engagement:        env("MARQ_ENGAGEMENT", "unspecified"),
	}
	if e := readContext(cfg.WorkDir); e != "" {
		cfg.Engagement = e
	}
	return cfg
}

// engagementContext is the on-disk form of the runtime-set engagement metadata.
type engagementContext struct {
	Engagement string `json:"engagement"`
}

func contextPath(workDir string) string { return filepath.Join(workDir, ".marq-context") }

// readContext returns the persisted engagement, or "" if the file is absent or
// unreadable (the env default then stands).
func readContext(workDir string) string {
	b, err := os.ReadFile(contextPath(workDir))
	if err != nil {
		return ""
	}
	var c engagementContext
	json.Unmarshal(b, &c)
	return c.Engagement
}

// SetEngagement records the engagement label/purpose, persisting it under
// WorkDir first so a failed write never leaves the live value ahead of the file
// and subsequent `marq run` processes inherit it. Operator is deliberately not
// settable here — it stays the human/env-set attribution anchor.
func SetEngagement(engagement string) error {
	engMu.Lock()
	defer engMu.Unlock()
	b, _ := json.Marshal(engagementContext{engagement})
	if err := os.WriteFile(contextPath(C.WorkDir), b, 0o644); err != nil {
		return err
	}
	C.Engagement = engagement
	return nil
}

// Attribution returns the current engagement label under a read lock — the
// race-free way to read the one mutable field of C. Callers on the concurrent
// path (audit records, banner, report rendering) must use this.
func Attribution() string {
	engMu.RLock()
	defer engMu.RUnlock()
	return C.Engagement
}

// C is the process-wide configuration, resolved once at startup. Components
// read config.C directly.
var C = Load()

// TrafficNote is the precise passive-only promise, shared with the MCP instructions.
const TrafficNote = "never scan, crawl or exploit a target's hosts — the only traffic that can reach a target's own infrastructure is ordinary DNS resolution hitting its authoritative nameservers. Username and email tools send lookup requests to third-party sites."

// Banner returns the usage notice shown to the operator before tools run.
// Pointer receiver + Attribution() so it doesn't read the mutable field while
// set_engagement may be writing it concurrently.
func (c *Config) Banner() string {
	if c.SkillsOnly {
		return "marq is running in knowledge-only mode: the cyber skills library " +
			"and advisory tools are available; the OSINT tool suite is not loaded. " +
			"Findings and file access are audit-logged.\n  operator   : " + c.Operator + "\n  audit log  : " + c.AuditLog
	}
	return fmt.Sprintf(
		"marq does passive OSINT only: its tools query public third-party sources "+
			"(DNS resolvers, WHOIS, certificate logs, Shodan/Censys, web archives, breach "+
			"databases, social and code-hosting sites) and %s People OSINT is "+
			"personal-data processing — keep it to the recorded purpose. Every OSINT-tool, "+
			"file and findings call is audit-logged; a tool won't run if its start record "+
			"can't be written.\n"+
			"  operator   : %s\n"+
			"  engagement : %s\n"+
			"  audit log  : %s",
		TrafficNote, c.Operator, Attribution(), c.AuditLog,
	)
}
