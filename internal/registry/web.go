package registry

import (
	"strconv"

	"github.com/rhaist/marq/internal/shellword"
)

const dirbCommon = "/usr/share/wordlists/dirb/common.txt"

// web returns the web-application testing tools.
func web() []Tool {
	return []Tool{
		{
			Name:   "nuclei",
			Active: true,
			Desc: "Run nuclei vulnerability templates against a URL/host. Optionally restrict to " +
				"`templates` (a tag or path) and/or `severity` (e.g. \"medium,high,critical\").",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "URL or host", Required: true},
				{Name: "templates", Type: StringParam, Desc: "template tag or path", Default: ""},
				{Name: "severity", Type: StringParam, Desc: "severity filter", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"nuclei", "-silent", "-u", a.S("target")}
				if a.S("templates") != "" {
					argv = append(argv, "-t", a.S("templates"))
				}
				if a.S("severity") != "" {
					argv = append(argv, "-severity", a.S("severity"))
				}
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "nikto",
			Active: true,
			Desc: "Run a Nikto web server scan against a URL or host. Runs in the background (a full " +
				"scan exceeds an interactive window and would blow the client call timeout) — poll with " +
				"list_jobs / job_status.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "URL or host", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"nikto", "-h", a.S("target")}, Target: a.S("target"), Background: true}
			},
		},
		{
			Name:   "ffuf",
			Active: true,
			Desc: "Content/directory fuzzing with ffuf. `url` must contain the FUZZ keyword " +
				"(e.g. https://host/FUZZ). Extra raw flags via `options`.",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "URL containing FUZZ", Required: true},
				{Name: "wordlist", Type: StringParam, Desc: "wordlist path", Default: dirbCommon},
				{Name: "options", Type: StringParam, Desc: "extra raw flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"ffuf", "-w", a.S("wordlist"), "-u", a.S("url")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name:   "gobuster_dir",
			Active: true,
			Desc:   "Directory brute force with gobuster against a base URL.",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "base URL", Required: true},
				{Name: "wordlist", Type: StringParam, Desc: "wordlist path", Default: dirbCommon},
			},
			Build: func(a Args) Invocation {
				argv := []string{"gobuster", "dir", "-q", "-u", a.S("url"), "-w", a.S("wordlist")}
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name:   "whatweb",
			Active: true,
			Desc:   "Fingerprint web technologies on a URL/host with WhatWeb.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "URL or host", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"whatweb", a.S("target")}, Target: a.S("target")}
			},
		},
		{
			Name:   "wpscan",
			Active: true,
			Desc: "Scan a WordPress site with WPScan. Default enumerates vulnerable plugins. Provide an " +
				"API token via WPSCAN_API_TOKEN for vuln data.",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "WordPress site URL", Required: true},
				{Name: "options", Type: StringParam, Desc: "raw wpscan flags", Default: "--enumerate vp"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"wpscan", "--url", a.S("url")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name:   "sqlmap",
			Active: true,
			Desc: "Test a URL for SQL injection with sqlmap. Always runs `--batch` (non-interactive); " +
				"add raw flags via `options`.",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "target URL", Required: true},
				{Name: "options", Type: StringParam, Desc: "raw sqlmap flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				// --batch is hard-coded (not in an overridable default) so passing
				// `options` can never drop it and leave sqlmap prompting -> hang.
				argv := []string{"sqlmap", "-u", a.S("url"), "--batch"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name:   "katana",
			Active: true,
			Desc: "Crawl a site for endpoints with katana (projectdiscovery). Discovers links, forms and " +
				"(with `js_crawl`) endpoints embedded in JavaScript. Actively requests the target. `depth` " +
				"bounds crawl recursion.",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "target URL", Required: true},
				{Name: "depth", Type: IntParam, Desc: "crawl depth", Default: 2},
				{Name: "js_crawl", Type: BoolParam, Desc: "crawl JS endpoints", Default: true},
			},
			Build: func(a Args) Invocation {
				argv := []string{"katana", "-u", a.S("url"), "-silent", "-d", strconv.Itoa(a.I("depth"))}
				if a.B("js_crawl") {
					argv = append(argv, "-jc")
				}
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name:   "feroxbuster",
			Active: true,
			Desc: "Fast recursive content/directory discovery with feroxbuster — a modern alternative to " +
				"gobuster/ffuf with automatic recursion. Extra raw flags via `options` (e.g. '-x php,txt -d 2').",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "target URL", Required: true},
				{Name: "wordlist", Type: StringParam, Desc: "wordlist path", Default: dirbCommon},
				{Name: "options", Type: StringParam, Desc: "extra raw flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"feroxbuster", "-u", a.S("url"), "-w", a.S("wordlist"), "--silent", "--no-state"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name:   "arjun",
			Active: true,
			Desc: "Discover hidden HTTP parameters on an endpoint with arjun. `method` is GET/POST/JSON/XML. " +
				"Useful before fuzzing for injection on params the app accepts but doesn't document.",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "endpoint URL", Required: true},
				{Name: "method", Type: StringParam, Desc: "GET/POST/JSON/XML", Default: "GET"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"arjun", "-u", a.S("url"), "-m", a.S("method")}
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name:   "dalfox",
			Active: true,
			Desc: "Scan a URL for XSS with dalfox. Tests reflected/stored/DOM vectors and verifies " +
				"findings. Add raw flags via `options` (e.g. a custom header or '--deep-domxss').",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "target URL", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra raw flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"dalfox", "url", a.S("url"), "--silence", "--no-color", "--no-spinner"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name:   "wafw00f",
			Active: true,
			Desc: "Detect and fingerprint a Web Application Firewall in front of a URL/host with wafw00f. " +
				"`-a` reports all matching WAF signatures.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "URL or host", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"wafw00f", "-a", a.S("target")}, Target: a.S("target")}
			},
		},
		{
			Name:   "testssl",
			Active: true,
			Desc: "Analyse a host's SSL/TLS configuration with testssl.sh: protocols, ciphers, cert chain " +
				"and known TLS vulnerabilities (Heartbleed, ROBOT, etc.). `host` is host:port (port " +
				"defaults to 443). Extra flags via `options`. Thorough — runs in the background; poll with " +
				"list_jobs / job_status.",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "host:port", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra raw flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"testssl", "--quiet", "--color", "0"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("host"))
				return Invocation{Argv: argv, Target: a.S("host"), Background: true}
			},
		},
		{
			Name:   "cmseek",
			Active: true,
			Desc: "Detect the CMS behind a site and known issues with CMSeeK (180+ CMSs, broader than " +
				"wpscan). Runs non-interactively in batch mode.",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "target URL", Required: true},
			},
			Build: func(a Args) Invocation {
				argv := []string{"cmseek", "--batch", "--follow-redirect", "-u", a.S("url")}
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name: "jwt_tool",
			Desc: "Analyse or attack a JSON Web Token with jwt_tool. `token` is the raw JWT. With no " +
				"`options` it decodes the header/claims and flags known weaknesses (alg:none, key " +
				"confusion, weak/expired signature). Add raw flags via `options`: '-X a' forges an " +
				"alg:none token, '-C -d <wordlist>' brute-forces the HMAC signing key, '-T' tampers " +
				"interactively (don't — no TTY). Operates on the token string only; no network.",
			Params: []Param{
				{Name: "token", Type: StringParam, Desc: "raw JWT", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra raw flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"jwt_tool", a.S("token")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("token")}
			},
		},
		{
			Name: "trivy",
			Desc: "Scan for vulnerabilities, exposed secrets and misconfigurations with Trivy. `mode` " +
				"selects the target type: fs (a filesystem path, stage it under /work), image (a " +
				"container image ref), repo (a git URL), or config (IaC / Dockerfiles). `target` is the " +
				"path/image/URL. Reports CVEs by severity.",
			Params: []Param{
				{Name: "mode", Type: StringParam, Desc: "fs/image/repo/config", Default: "fs"},
				{Name: "target", Type: StringParam, Desc: "path / image ref / repo URL", Required: true},
			},
			Build: func(a Args) Invocation {
				argv := []string{"trivy", a.S("mode"), "--quiet", "--scanners", "vuln,secret,misconfig", a.S("target")}
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name: "interactsh",
			Desc: "Start an Interactsh out-of-band (OOB) listener for detecting BLIND vulnerabilities " +
				"(blind SQLi, SSRF, RCE, XXE, blind XSS). Runs in the BACKGROUND and returns a job dir. " +
				"The generated callback domain is printed at the top of the job's stdout.log — read it " +
				"with job_status, embed that domain in payloads, then poll job_status again: any " +
				"DNS/HTTP/SMTP callback to it is logged back, proving the payload fired. Uses a public " +
				"interactsh server by default; override with a self-hosted `server`.",
			Params: []Param{
				{Name: "server", Type: StringParam, Desc: "self-hosted interactsh server URL", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"interactsh-client", "-v"}
				if a.S("server") != "" {
					argv = append(argv, "-server", a.S("server"))
				}
				return Invocation{Argv: argv, Target: "interactsh", Background: true}
			},
		},
		{
			Name: "paramspider",
			Desc: "Discover hidden injectable parameters for a domain with paramspider. " +
				"Crawls and extracts URLs with parameters, excluding common noise. `domain` " +
				"is the target domain. NB: results are written to /work/results/<domain>.txt (not " +
				"stdout) — read them back with read_file.",
			Params: []Param{
				{Name: "domain", Type: StringParam, Desc: "target domain", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra paramspider flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"paramspider", "-d", a.S("domain")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("domain")}
			},
		},
		{
			Name:   "sstimap",
			Active: true,
			Desc: "Detect and exploit Server-Side Template Injection (SSTI) with sstimap. `url` " +
				"is the target URL. Use `options` for method, parameters, and template engine " +
				"hints (e.g. -d data.txt -m POST -p name).",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "target URL", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra sstimap flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"sstimap", "-u", a.S("url")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
	}
}
