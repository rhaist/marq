// Package findings is the engagement deliverable store (feature #1). Tools and
// the agent record structured findings as they go; render produces the Markdown
// + CSV report. Without this, tool output evaporates into the chat and the
// engagement leaves no artifact behind.
package findings

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"pentest-mcp/internal/audit"
	"pentest-mcp/internal/config"
)

func nowTS() string { return time.Now().UTC().Format(time.RFC3339) }

// Finding is one recorded result. Stored as a JSON line in findings.jsonl.
type Finding struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Severity       string `json:"severity"`
	Target         string `json:"target"`
	Evidence       string `json:"evidence"`
	Recommendation string `json:"recommendation"`
	TS             string `json:"ts"`
}

// severityRank orders findings for the report (most severe first).
var severityRank = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4}

func normalizeSeverity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if _, ok := severityRank[s]; ok {
		return s
	}
	return "info"
}

func storePath() string { return filepath.Join(config.C.WorkDir, "findings.jsonl") }

// load reads all findings from the store (empty slice if none yet).
func load() ([]Finding, error) {
	fh, err := os.Open(storePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer fh.Close()
	var out []Finding
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var f Finding
		if err := json.Unmarshal([]byte(line), &f); err == nil {
			out = append(out, f)
		}
	}
	return out, sc.Err()
}

// Report appends a structured finding and returns a confirmation. severity is
// normalized to info/low/medium/high/critical; only title is required.
func Report(title, severity, target, evidence, recommendation string) string {
	id := audit.LogStart("report_finding", target, []string{"report_finding", title})
	out, err := func() (string, error) {
		if strings.TrimSpace(title) == "" {
			return "error: a finding needs a title", nil
		}
		existing, _ := load()
		f := Finding{
			ID:             fmt.Sprintf("F-%03d", len(existing)+1),
			Title:          strings.TrimSpace(title),
			Severity:       normalizeSeverity(severity),
			Target:         target,
			Evidence:       evidence,
			Recommendation: recommendation,
			TS:             nowTS(),
		}
		if err := os.MkdirAll(config.C.WorkDir, 0o755); err != nil {
			return "", err
		}
		fh, err := os.OpenFile(storePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		defer fh.Close()
		line, _ := json.Marshal(f)
		if _, err := fh.Write(append(line, '\n')); err != nil {
			return "", err
		}
		_ = fh.Sync()
		return fmt.Sprintf("recorded %s [%s] %q (target: %s). %d findings so far. "+
			"Call render_report to write the Markdown/CSV report.",
			f.ID, f.Severity, f.Title, f.Target, len(existing)+1), nil
	}()
	endAudit(id, "report_finding", err)
	if err != nil {
		return "error: " + err.Error()
	}
	return out
}

// RenderReport writes findings.md and findings.csv (severity-sorted) to the work
// dir and returns a summary.
func RenderReport() string {
	id := audit.LogStart("render_report", "", []string{"render_report"})
	out, err := func() (string, error) {
		all, err := load()
		if err != nil {
			return "", err
		}
		if len(all) == 0 {
			return "no findings recorded yet — use report_finding first", nil
		}
		sort.SliceStable(all, func(i, j int) bool {
			return severityRank[all[i].Severity] < severityRank[all[j].Severity]
		})
		mdPath := filepath.Join(config.C.WorkDir, "findings.md")
		csvPath := filepath.Join(config.C.WorkDir, "findings.csv")
		if err := os.WriteFile(mdPath, []byte(renderMarkdown(all)), 0o644); err != nil {
			return "", err
		}
		if err := writeCSV(csvPath, all); err != nil {
			return "", err
		}
		counts := map[string]int{}
		for _, f := range all {
			counts[f.Severity]++
		}
		return fmt.Sprintf("wrote %s and %s — %d findings (%s).",
			mdPath, csvPath, len(all), summarizeCounts(counts)), nil
	}()
	endAudit(id, "render_report", err)
	if err != nil {
		return "error: " + err.Error()
	}
	return out
}

func renderMarkdown(all []Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Penetration Test Findings\n\n")
	fmt.Fprintf(&b, "_Operator: %s · Engagement: %s_\n\n", config.C.Operator, config.C.Engagement)
	counts := map[string]int{}
	for _, f := range all {
		counts[f.Severity]++
	}
	fmt.Fprintf(&b, "**%d findings** — %s\n\n", len(all), summarizeCounts(counts))
	for _, f := range all {
		fmt.Fprintf(&b, "## %s — %s (%s)\n\n", f.ID, f.Title, strings.ToUpper(f.Severity))
		fmt.Fprintf(&b, "- **Target:** %s\n", f.Target)
		fmt.Fprintf(&b, "- **Recorded:** %s\n\n", f.TS)
		if f.Evidence != "" {
			fmt.Fprintf(&b, "**Evidence**\n\n```\n%s\n```\n\n", f.Evidence)
		}
		if f.Recommendation != "" {
			fmt.Fprintf(&b, "**Recommendation**\n\n%s\n\n", f.Recommendation)
		}
	}
	return b.String()
}

func writeCSV(path string, all []Finding) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	w := csv.NewWriter(fh)
	defer w.Flush()
	_ = w.Write([]string{"id", "severity", "title", "target", "recommendation", "ts"})
	for _, f := range all {
		_ = w.Write([]string{f.ID, f.Severity, f.Title, f.Target, f.Recommendation, f.TS})
	}
	return w.Error()
}

func summarizeCounts(counts map[string]int) string {
	order := []string{"critical", "high", "medium", "low", "info"}
	var parts []string
	for _, s := range order {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[s], s))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func endAudit(id, op string, err error) {
	code := 0
	msg := ""
	if err != nil {
		code = 1
		msg = err.Error()
	}
	c := code
	audit.LogEnd(id, op, &c, 0, false, msg)
}
