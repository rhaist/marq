package registry

import (
	_ "embed"

	"pentest-mcp/internal/findings"
	"pentest-mcp/internal/jobs"
)

// Methodology is the engagement workflow guidance (feature #3). It is exposed as
// the pentest://methodology MCP resource (serve mode) and injected into the
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
				"response) and a `recommendation`. Call render_report when done to write the report.",
			Params: []Param{
				{Name: "title", Type: StringParam, Desc: "short finding title", Required: true},
				{Name: "severity", Type: StringParam, Desc: "info/low/medium/high/critical", Default: "info"},
				{Name: "target", Type: StringParam, Desc: "affected host/URL/asset", Default: ""},
				{Name: "evidence", Type: StringParam, Desc: "proof: tool output, request/response", Default: ""},
				{Name: "recommendation", Type: StringParam, Desc: "remediation advice", Default: ""},
			},
			Handler: func(a Args) string {
				return findings.Report(a.S("title"), a.S("severity"), a.S("target"), a.S("evidence"), a.S("recommendation"))
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
