package registry

import (
	"strings"

	"marq/internal/shellword"
)

// internal returns Active Directory and internal-network pentest tools.
// These replaced metasploit — each is a non-interactive CLI, far lighter than
// the 1.8 GB mingw/postgres subtree metasploit dragged in. Every call is
// audit-logged like the rest of the suite.
func internal() []Tool {
	return []Tool{
		{
			Name:   "impacket_secretsdump",
			Active: true,
			Desc: "Dump password hashes from NTDS.dit / SAM / LSASS remotely or from a " +
				"local file with impacket-secretsdump. Provide `target` as either a local " +
				"file path or 'domain/user:password@host' for remote DRSUAPI dump. " +
				"Outputs NTLM/Kerberos hashes ready for john/hashcat. High impact.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "local file or domain/user:pass@host", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra secretsdump flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"impacket-secretsdump"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "impacket_kerberoast",
			Active: true,
			Desc: "Kerberoasting with impacket-GetUserSPNs: request TGS tickets for SPN " +
				"accounts in a domain and extract hashcat/john-crackable hashes. " +
				"Provide `target` as 'domain/user:password@host'. High impact — " +
				"authorized targets only.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain/user:pass@DC", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra flags (e.g. -request-user svcacct)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"impacket-GetUserSPNs"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "impacket_asreproast",
			Active: true,
			Desc: "AS-REP roasting with impacket-GetNPUsers: request AS-REP hashes for " +
				"accounts with 'Do not require Kerberos preauthentication' set. " +
				"Provide `target` as 'domain/user:password@host'. Pass a user list via " +
				"`options` (e.g. -usersfile /work/users.txt). High impact.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain/user:pass@DC", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra flags (e.g. -usersfile path)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"impacket-GetNPUsers"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "impacket_psexec",
			Active: true,
			Desc: "Execute a command on a remote Windows host via PsExec-style SMB named " +
				"pipe (impacket-psexec). `target` is 'domain/user:password@host' (or " +
				"'user:hash@host' with -hashes). `command` is the shell command to run. " +
				"High impact — lateral movement, authorized targets only.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain/user:pass@host", Required: true},
				{Name: "command", Type: StringParam, Desc: "command to execute remotely", Default: "whoami"},
				{Name: "options", Type: StringParam, Desc: "extra flags (e.g. -hashes :NTLM)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"impacket-psexec"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"), a.S("command"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "impacket_wmiexec",
			Active: true,
			Desc: "Execute a command on a remote Windows host via WMI (impacket-wmiexec). " +
				"Stealthier than psexec (no service dropped). `target` is " +
				"'domain/user:password@host' (or pass -hashes for pass-the-hash). " +
				"High impact — lateral movement, authorized targets only.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain/user:pass@host", Required: true},
				{Name: "command", Type: StringParam, Desc: "command to execute remotely", Default: "whoami"},
				{Name: "options", Type: StringParam, Desc: "extra flags (e.g. -hashes :NTLM)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"impacket-wmiexec"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"), a.S("command"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "impacket_ntlmrelayx",
			Active: true,
			Desc: "NTLM relay attack with impacket-ntlmrelayx: relay captured NTLM auth " +
				"to targets in a file. Stage a target list with write_file then pass " +
				"the path via `options` (-tf /work/targets.txt). Runs in the BACKGROUND " +
				"and returns a job dir — poll with job_status. High impact.",
			Params: []Param{
				{Name: "options", Type: StringParam, Desc: "extra flags (e.g. -tf targets.txt -smb2support)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"impacket-ntlmrelayx"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: "ntlmrelayx", Background: true}
			},
		},
		{
			Name:   "netexec",
			Active: true,
			Desc: "NetExec (nxc, CrackMapExec successor) — enumerate and attack SMB/WinRM/" +
				"SSH/LDAP/MSSQL/WMI/FTP. `target` is a host or comma-separated list. " +
				"`protocol` selects the module (smb/winrm/ssh/ldap/mssql/wmi/ftp). Pass " +
				"creds via `options` (-u user -p pass). High impact — authorized only.",
			Params: []Param{
				{Name: "protocol", Type: StringParam, Desc: "smb/winrm/ssh/ldap/mssql/wmi/ftp", Required: true},
				{Name: "target", Type: StringParam, Desc: "host or comma-separated list", Required: true},
				{Name: "options", Type: StringParam, Desc: "raw nxc flags (-u -p -d ...)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"netexec", a.S("protocol")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "certipy_find",
			Active: true,
			Desc: "Enumerate Active Directory Certificate Services (AD CS) with certipy. " +
				"Discovers vulnerable certificate templates (ESC1-ESC17), CAs, and " +
				"enrollment endpoints. `target` is 'domain/user:password@DC'. " +
				"Outputs JSON + text findings.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain/user:pass@DC", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra certipy find flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				domain, user, password, host := parseImpacketTarget(a.S("target"))
				if domain != "" {
					user += "@" + domain
				}
				argv := []string{"certipy-ad", "find"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, "-u", user, "-p", password)
				if host != "" {
					argv = append(argv, "-dc-ip", host)
				}
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "bloodhound_collect",
			Active: true,
			Desc: "Collect Active Directory attack-path data with bloodhound-python. " +
				"Outputs JSON files for ingestion by BloodHound. `target` is " +
				"'domain/user:password@DC'. `collection` selects the data to gather " +
				"(All/Group/Users/Computers/LocalAdmins/ACLs/DCOM/RDP/PSRemote/Session). " +
				"Requires DNS resolution to the DC — pass `-ns <DC-IP>` via options.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "domain/user:pass@DC", Required: true},
				{Name: "collection", Type: StringParam, Desc: "All/Group/Users/Computers/...", Default: "All"},
				{Name: "options", Type: StringParam, Desc: "extra flags (e.g. -ns DC-IP)", Default: ""},
			},
			Build: func(a Args) Invocation {
				domain, user, password, host := parseImpacketTarget(a.S("target"))
				argv := []string{"bloodhound-python", "-u", user, "-p", password, "-d", domain, "-c", a.S("collection")}
				if host != "" {
					argv = append(argv, "-ns", host)
				}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "evil_winrm",
			Active: true,
			Desc: "Connect to a Windows host via WinRM and execute a command (evil-winrm). " +
				"`target` is the host:port. Pass credentials via `options` " +
				"(-u user -p pass, or -u user -H NTLM-hash, or -S for SSL). " +
				"Supports file upload/download. High impact — authorized only.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "host:port", Required: true},
				{Name: "options", Type: StringParam, Desc: "raw evil-winrm flags (-u -p -H -S ...)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"evil-winrm", "-i", a.S("target")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "enum4linux",
			Active: true,
			Desc: "Enumerate Windows/Samba hosts via SMB/RPC/SAMR with enum4linux-ng " +
				"(modern Python rewrite). Discovers shares, users, groups, password " +
				"policy, null-session access. `target` is a host IP. The single best " +
				"first move for internal network Windows enumeration.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "host IP", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"enum4linux-ng"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("target"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "smb_enum",
			Active: true,
			Desc: "Enumerate SMB shares and access permissions with smbmap. `target` is a " +
				"host IP. Defaults to anonymous (null-session) enumeration; pass creds " +
				"via `options` (-u user -p pass -d domain). Lists share names, access " +
				"level, and comments.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "host IP", Required: true},
				{Name: "options", Type: StringParam, Desc: "raw smbmap flags (-u -p -d ...)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"smbmap", "-H", a.S("target")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "ldap_search",
			Active: true,
			Desc: "Query an LDAP directory (e.g. Active Directory) with ldapsearch. " +
				"`target` is the LDAP server (host or ldap://host). `filter` is the LDAP " +
				"filter (e.g. '(objectclass=user)'). `options` carries bind DN, password, " +
				"and search base (e.g. -x -D 'cn=admin,dc=...' -w pass -b 'dc=...').",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "LDAP server host or URL", Required: true},
				{Name: "filter", Type: StringParam, Desc: "LDAP filter", Default: "(objectclass=*)"},
				{Name: "options", Type: StringParam, Desc: "raw ldapsearch flags (-x -D -w -b ...)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"ldapsearch", "-H", a.S("target")}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("filter"))
				return Invocation{Argv: argv, Target: a.S("target")}
			},
		},
		{
			Name:   "responder",
			Active: true,
			Desc: "Start Responder for LLMNR/NBT-NS/mDNS poisoning to capture NTLMv2 " +
				"challenge/response hashes on the local network segment. Runs in the " +
				"BACKGROUND and returns a job dir — poll with job_status for captured " +
				"hashes. High impact — authorized networks only.",
			Params: []Param{
				{Name: "options", Type: StringParam, Desc: "raw responder flags (e.g. -I eth0 -rdw)", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"responder"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				return Invocation{Argv: argv, Target: "responder", Background: true}
			},
		},
		{
			Name:   "nbtscan",
			Active: true,
			Desc: "Scan a network for NetBIOS name information with nbtscan. `target` is a " +
				"CIDR or comma-separated host list. Returns NetBIOS names, workgroups, " +
				"and MAC addresses — fast recon for Windows networks.",
			Params: []Param{
				{Name: "target", Type: StringParam, Desc: "CIDR or host list", Required: true},
			},
			Build: func(a Args) Invocation {
				return Invocation{Argv: []string{"nbtscan", a.S("target")}, Target: a.S("target")}
			},
		},
	}
}

// parseImpacketTarget splits an impacket-style "domain/user:password@host"
// connection string into its parts. Every part is optional; callers append
// only the flags they actually got, so a missing piece yields "" not a panic.
func parseImpacketTarget(t string) (domain, user, password, host string) {
	creds := t
	if at := strings.LastIndex(t, "@"); at >= 0 {
		creds, host = t[:at], t[at+1:]
	}
	if slash := strings.Index(creds, "/"); slash >= 0 {
		domain, creds = creds[:slash], creds[slash+1:]
	}
	user = creds
	if colon := strings.Index(creds, ":"); colon >= 0 {
		user, password = creds[:colon], creds[colon+1:]
	}
	return
}
