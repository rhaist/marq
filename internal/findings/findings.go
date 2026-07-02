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

	"marq/internal/audit"
	"marq/internal/config"
)

func nowTS() string { return time.Now().UTC().Format(time.RFC3339) }

// Finding is one recorded result. Stored as a JSON line in findings.jsonl.
type Finding struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Severity       string  `json:"severity"`
	Target         string  `json:"target"`
	Evidence       string  `json:"evidence"`
	Recommendation string  `json:"recommendation"`
	CVSS           string  `json:"cvss,omitempty"`        // CVSS 3.1 vector
	CVSSScore      float64 `json:"cvss_score,omitempty"`  // computed base score
	CVSSRating     string  `json:"cvss_rating,omitempty"` // rating derived from the vector
	CWE            string  `json:"cwe,omitempty"`         // e.g. CWE-89
	References     string  `json:"references,omitempty"`  // links / CVE ids
	TS             string  `json:"ts"`
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

// Report appends a structured finding and returns a confirmation. Only title is
// required.
//
// Severity precedence: the caller's explicit severity is authoritative (it's the
// human-meaningful risk call). A CVSS 3.1 vector, if valid, is computed and kept
// as supporting detail — it only *sets* severity when the caller omitted one. If
// both are given and disagree, the caller's severity stands and the divergence is
// flagged rather than silently overwritten (a model-guessed vector must not mask
// the assigned risk).
func Report(title, severity, target, evidence, recommendation, cvss, cwe, references string) string {
	return withAudit("report_finding", target, []string{"report_finding", title}, func() (string, error) {
		if strings.TrimSpace(title) == "" {
			return "error: a finding needs a title", nil
		}
		var score float64
		var cvssRating string
		if v := strings.TrimSpace(cvss); v != "" {
			if s, rated, valid := cvssBase(v); valid {
				score, cvssRating = s, rated
			}
		}
		explicit := strings.TrimSpace(severity) != ""
		var sev string
		switch {
		case explicit:
			sev = normalizeSeverity(severity)
		case cvssRating != "":
			sev = normalizeSeverity(cvssRating) // derive only when the caller omitted severity; "none" -> info, not rank-0
		default:
			sev = "info"
		}
		mismatch := explicit && cvssRating != "" && cvssRating != sev

		existing, _ := load()
		f := Finding{
			ID:             fmt.Sprintf("F-%03d", len(existing)+1),
			Title:          strings.TrimSpace(title),
			Severity:       sev,
			Target:         target,
			Evidence:       evidence,
			Recommendation: recommendation,
			CVSS:           strings.TrimSpace(cvss),
			CVSSScore:      score,
			CVSSRating:     cvssRating,
			CWE:            strings.TrimSpace(cwe),
			References:     strings.TrimSpace(references),
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
		note := ""
		if mismatch {
			note = fmt.Sprintf(" (note: assigned severity %s, but the CVSS vector rates %s %.1f — kept your severity)",
				f.Severity, f.CVSSRating, f.CVSSScore)
		}
		return fmt.Sprintf("recorded %s [%s] %q (target: %s)%s. %d findings so far. "+
			"Call render_report to write the Markdown/CSV report.",
			f.ID, f.Severity, f.Title, f.Target, note, len(existing)+1), nil
	})
}

// RenderReport writes findings.md and findings.csv (severity-sorted) to the work
// dir and returns a summary.
func RenderReport() string {
	return withAudit("render_report", "", []string{"render_report"}, func() (string, error) {
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
	})
}

func renderMarkdown(all []Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Penetration Test Findings\n\n")
	engagement, _ := config.Attribution()
	fmt.Fprintf(&b, "_Operator: %s · Engagement: %s_\n\n", config.C.Operator, engagement)
	counts := map[string]int{}
	for _, f := range all {
		counts[f.Severity]++
	}
	fmt.Fprintf(&b, "**%d findings** — %s\n\n", len(all), summarizeCounts(counts))
	for _, f := range all {
		fmt.Fprintf(&b, "## %s — %s (%s)\n\n", f.ID, f.Title, strings.ToUpper(f.Severity))
		fmt.Fprintf(&b, "- **Target:** %s\n", f.Target)
		if f.CVSS != "" {
			fmt.Fprintf(&b, "- **CVSS:** %.1f %s (`%s`)", f.CVSSScore, f.CVSSRating, f.CVSS)
			if f.CVSSRating != "" && f.CVSSRating != f.Severity {
				fmt.Fprintf(&b, " — _CVSS rating differs from assigned severity (%s)_", f.Severity)
			}
			b.WriteString("\n")
		}
		if f.CWE != "" {
			fmt.Fprintf(&b, "- **CWE:** %s\n", f.CWE)
		}
		if f.References != "" {
			fmt.Fprintf(&b, "- **References:** %s\n", f.References)
		}
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
	_ = w.Write([]string{"id", "severity", "cvss_score", "cvss", "cwe", "title", "target", "recommendation", "ts"})
	for _, f := range all {
		cvssScore := ""
		if f.CVSSScore > 0 {
			cvssScore = fmt.Sprintf("%.1f", f.CVSSScore)
		}
		row := []string{f.ID, f.Severity, cvssScore, f.CVSS, f.CWE, f.Title, f.Target, f.Recommendation, f.TS}
		for i, v := range row {
			row[i] = csvSanitize(v)
		}
		_ = w.Write(row)
	}
	return w.Error()
}

// csvSanitize defuses spreadsheet formula injection: a field a model or target
// controls that begins with =, +, -, @, tab, or CR is treated as a formula by
// Excel/Sheets on open. Prefix it with a single quote so it renders as text.
func csvSanitize(s string) string {
	if s == "" {
		return s
	}
	if strings.IndexByte("=+-@\t\r", s[0]) >= 0 {
		return "'" + s
	}
	return s
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

// withAudit wraps fn in the same start/end audit envelope used by Report and
// RenderReport. A soft error (returned with a nil err) passes straight through
// to the model; a hard error (non-nil err) is surfaced as "error: <msg>".
func withAudit(op, target string, argv []string, fn func() (string, error)) string {
	id, _ := audit.LogStart(op, target, argv)
	out, err := fn()
	endAudit(id, op, err)
	if err != nil {
		return "error: " + err.Error()
	}
	return out
}
