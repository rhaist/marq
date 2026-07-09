package registry

import (
	_ "embed"

	"github.com/rhaist/marq/internal/findings"
	"github.com/rhaist/marq/internal/jobs"
	"github.com/rhaist/marq/internal/skills"
)

// knowledgeTools returns the on-demand skills library tool.
func knowledgeTools() []Tool {
	return []Tool{
		{
			Name: "load_skill",
			Desc: "Load an expert playbook (offensive, malware, threat-intel, and governance — GRC, " +
				"standards, CISO) before working that kind of task. Pass one skill name or a comma list (max 5). Available skills:\n" +
				skills.IndexText(),
			Params: []Param{
				{Name: "name", Type: StringParam, Desc: "skill name(s), comma-separated (max 5); omit to list all skills"},
			},
			Handler: func(a Args) string { return skills.LoadMany(a.S("name")) },
		},
	}
}

// Methodology is the engagement workflow guidance (feature #3). It is exposed as
// the marq://methodology MCP resource (serve mode) and injected into the
// agent system prompt (tui mode).
//
//go:embed methodology.md
var Methodology string

// reportTools returns the findings (feature #1) and background-job (feature #2)
// tools. These are in-process Handler tools wired into the same registry, so MCP
// clients and the TUI agent both get them.
func reportTools() []Tool {
	return []Tool{
		{
			Name: "report_finding",
			Desc: "Record a validated security finding in the engagement deliverable. `severity` is one " +
				"of info/low/medium/high/critical. Provide `evidence` (the proof — tool output, request/" +
				"response) and a `recommendation`. Optionally add a CVSS 3.1 `cvss` vector (e.g. " +
				"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H — its base score is computed and sets severity), " +
				"a `cwe` (e.g. CWE-89), and `references` (CVE ids / links). Call render_report when done.",
			Params: []Param{
				{Name: "title", Type: StringParam, Desc: "short finding title", Required: true},
				{Name: "severity", Type: StringParam, Desc: "info/low/medium/high/critical (your call; if omitted and a cvss vector is given, it's derived from CVSS)", Default: ""},
				{Name: "target", Type: StringParam, Desc: "affected host/URL/asset", Default: ""},
				{Name: "evidence", Type: StringParam, Desc: "proof: tool output, request/response", Default: ""},
				{Name: "recommendation", Type: StringParam, Desc: "remediation advice", Default: ""},
				{Name: "cvss", Type: StringParam, Desc: "CVSS 3.1 vector string (optional)", Default: ""},
				{Name: "cwe", Type: StringParam, Desc: "CWE id, e.g. CWE-89 (optional)", Default: ""},
				{Name: "references", Type: StringParam, Desc: "CVE ids / links (optional)", Default: ""},
			},
			Handler: func(a Args) string {
				return findings.Report(a.S("title"), a.S("severity"), a.S("target"), a.S("evidence"),
					a.S("recommendation"), a.S("cvss"), a.S("cwe"), a.S("references"))
			},
		},
		{
			Name: "render_report",
			Desc: "Write the engagement report (findings.md + findings.csv, severity-sorted) into the " +
				"working area from all recorded findings. Call after recording findings with report_finding.",
			Params:  []Param{},
			Handler: func(a Args) string { return findings.RenderReport() },
		},
		{
			Name: "list_jobs",
			Desc: "List background jobs (e.g. spiderfoot) and whether each is running or done. Background " +
				"tools return a job dir; this is the quick way to see their state without polling files.",
			Params:  []Param{},
			Handler: func(a Args) string { return jobs.List() },
		},
		{
			Name: "job_status",
			Desc: "Report a background job's state (running/done + exit code) and a tail of its output. " +
				"`job` is the job-dir name or path returned when the background tool was launched.",
			Params: []Param{
				{Name: "job", Type: StringParam, Desc: "job-dir name or path", Required: true},
			},
			Handler: func(a Args) string { return jobs.Status(a.S("job")) },
		},
	}
}
