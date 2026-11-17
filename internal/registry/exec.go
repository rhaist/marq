package registry

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rhaist/marq/internal/shellword"
)

// execAllow is what the generic executor may run: the image's security tools,
// no shells, interpreters or tools that run arbitrary commands (radare2 -c '!…',
// swaks --pipe), so arguments stay visible to the scope check. Drift protection,
// not a sandbox: the container is the sandbox.
var execAllow = []string{
	"arjun", "bloodhound-python", "cdncheck", "certipy-ad", "cmseek", "curl", "dalfox",
	"dnsrecon", "dnsx", "enum4linux-ng", "evil-winrm", "feroxbuster", "ffuf", "fierce", "fping",
	"gobuster", "gospider", "httpx-toolkit", "hydra", "interactsh-client", "jwt_tool", "katana",
	"ldapsearch", "masscan", "naabu", "nbtscan", "netexec", "nikto", "nmap", "nuclei", "nxc",
	"onesixtyone", "paramspider", "responder", "smbclient", "smbmap", "smtp-user-enum",
	"snmp-check", "snmpwalk", "sqlmap", "ssh-audit", "sslscan", "sstimap", "subfinder",
	"testssl", "wafw00f", "whatweb", "wpscan",
}

// execOffline touch no engagement target (local files, or public data sources),
// so they run without a scope.
var execOffline = []string{
	"capa", "dig", "exiftool", "gitleaks", "hashcat", "john", "nth", "olevba", "rabin2",
	"searchsploit", "trivy", "trufflehog", "whois", "yara",
}

// deniedFlag returns why a flag can't be used through exec: it targets hosts
// the scope check can't see (random or resumed scans, config files) or runs
// custom code (nmap script files, nuclei code templates).
func deniedFlag(bin, arg string) string {
	name, val, _ := strings.Cut(arg, "=")
	switch {
	case name == "-config" || name == "--config" || (bin == "curl" && name == "-K") || (bin == "masscan" && (name == "-c" || name == "--conf")):
		return "config files can carry hidden targets"
	case name == "--resume" || name == "--datadir" || (bin == "nmap" && name == "-iR"):
		return "it targets hosts the scope check can't see"
	case bin == "nuclei" && name == "-code":
		return "code templates run arbitrary commands"
	case bin == "nmap" && strings.HasPrefix(name, "--script") && strings.ContainsAny(val, "/.") && name != "--script-args":
		return "custom script files run arbitrary code; use named NSE scripts"
	}
	return ""
}

func execTools() []Tool {
	return []Tool{
		{
			Name:   "exec",
			Active: true,
			Desc: "Run an allowlisted security tool directly with your own arguments (argv only, no shell: " +
				"no pipes, redirects or $()). Use it for options or tools the dedicated wrappers don't " +
				"cover; prefer the wrapper when one fits. Every host, IP, CIDR and URL in the command must " +
				"be inside the engagement scope or the call is refused; target-list files (-iL, -l) are checked " +
				"too. Allowed: " + strings.Join(execAllow, ", ") + ", impacket-*; offline (no scope needed): " +
				strings.Join(execOffline, ", ") + ". Audit-logged.",
			Params: []Param{
				{Name: "command", Type: StringParam, Desc: "the command line, e.g. `nmap -sV -p 22,443 10.0.0.5`", Required: true},
				{Name: "background", Type: BoolParam, Desc: "run detached for long scans; poll with job_status", Default: false},
				{Name: "timeout", Type: IntParam, Desc: "seconds (0 = default, capped by MARQ_MAX_TIMEOUT)", Default: 0},
			},
			Build: func(a Args) Invocation {
				argv := shellword.Split(a.S("command"))
				if len(argv) == 0 {
					return Invocation{Refuse: "empty command"}
				}
				bin := filepath.Base(argv[0])
				offline := slices.Contains(execOffline, bin)
				if !offline && !slices.Contains(execAllow, bin) && !strings.HasPrefix(bin, "impacket-") {
					return Invocation{Argv: argv, Refuse: fmt.Sprintf("%q is not on the exec allowlist "+
						"(security tools only; no shells or interpreters). See `marq tools exec`.", bin)}
				}
				for i, arg := range argv[1:] {
					// nmap --script takes its value as the next arg too
					if arg == "--script" && bin == "nmap" && i+2 < len(argv) {
						arg += "=" + argv[i+2]
					}
					if why := deniedFlag(bin, arg); why != "" {
						return Invocation{Argv: argv, Refuse: fmt.Sprintf("%s %s is not allowed through exec: %s.", bin, arg, why)}
					}
				}
				argv[0] = bin // resolve from PATH, never a caller-supplied path
				return Invocation{Argv: argv, Target: a.S("command"), Timeout: a.I("timeout"),
					Background: a.B("background"), Passive: offline}
			},
		},
	}
}
