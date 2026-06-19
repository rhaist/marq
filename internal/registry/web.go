package registry

import (
	"strconv"

	"marq/internal/shellword"
)

const dirbCommon = "/usr/share/wordlists/dirb/common.txt"

// web returns the web-application testing tools.
func web() []Tool {
	return []Tool{
		{
			Name: "nuclei",
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
			Name: "nikto",
			Desc: "Run a Nikto web server scan against a URL or host.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "URL or host", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"nikto", "-h", a.S("target")}, Target: a.S("target")}
			},
		},
		{
			Name: "ffuf",
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
			Name: "gobuster_dir",
			Desc: "Directory brute force with gobuster against a base URL.",
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
			Name: "whatweb",
			Desc: "Fingerprint web technologies on a URL/host with WhatWeb.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "URL or host", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"whatweb", a.S("target")}, Target: a.S("target")}
			},
		},
		{
			Name: "wpscan",
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
			Name: "sqlmap",
			Desc: "Test a URL for SQL injection with sqlmap. `--batch` runs non-interactively with " +
				"defaults. Add raw flags via `options`.",
			Params: []Param{
				{Name: "url", Type: StringParam, Desc: "target URL", Required: true},
				{Name: "options", Type: StringParam, Desc: "raw sqlmap flags", Default: "--batch"},
			},
			Build: func(a Args) Invocation {
				argv := []string{"sqlmap", "-u", a.S("url")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("url")}
			},
		},
		{
			Name: "katana",
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
			Name: "feroxbuster",
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
			Name: "arjun",
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
			Name: "dalfox",
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
			Name: "wafw00f",
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
			Name: "testssl",
			Desc: "Analyse a host's SSL/TLS configuration with testssl.sh: protocols, ciphers, cert chain " +
				"and known TLS vulnerabilities (Heartbleed, ROBOT, etc.). `host` is host:port (port " +
				"defaults to 443). Extra flags via `options`. Thorough — can take a while.",
			Params: []Param{
				{Name: "host", Type: StringParam, Desc: "host:port", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra raw flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"testssl", "--quiet", "--color", "0"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("host"))
				return Invocation{Argv: argv, Target: a.S("host")}
			},
		},
		{
			Name: "cmseek",
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
	}
}
