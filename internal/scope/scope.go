// Package scope turns the free-text engagement scope note into a set of
// authorized targets and finds the targets an active command would touch, so
// registry.Call can refuse out-of-scope work before it runs. It guards against
// an agent drifting off-scope, not against a malicious operator: whoever can
// call set_engagement (or confirm an elicitation) can widen scope.
//
// Extraction fails closed: every token, flags included, is scanned for URLs,
// IPv4 addresses and ranges, IPv6 addresses and hostnames, and anything
// target-shaped that the set doesn't explicitly allow is out of scope.
package scope

import (
	"context"
	"net"
	"net/netip"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/publicsuffix"
)

// rules are IP prefixes, exact hosts and wildcard domains.
type rules struct {
	Prefixes []netip.Prefix
	Hosts    map[string]bool // exact hostnames, lowercase
	Domains  []string        // from "*.acme.com": acme.com and every subdomain
}

// Set is the parsed scope: authorized rules plus exclusions ("except
// vpn.acme.com", "!10.0.0.5") that win over everything else.
type Set struct {
	rules
	Excluded rules
}

// Empty reports whether no target is authorized.
func (s Set) Empty() bool { return len(s.Prefixes) == 0 && len(s.Hosts) == 0 && len(s.Domains) == 0 }

var exclusionWords = map[string]bool{"except": true, "excluding": true, "exclude": true, "not": true, "without": true, "but": true}

// Parse extracts targets from a scope note such as
// "*.acme.com except vpn.acme.com, 203.0.113.0/24, app.example.org — per SOW".
// Prose is ignored. Within a comma/semicolon/newline segment, everything after
// except/excluding/not/without is an exclusion; so is a "!"-prefixed entry.
func Parse(note string) Set {
	s := Set{rules: rules{Hosts: map[string]bool{}}, Excluded: rules{Hosts: map[string]bool{}}}
	for _, seg := range strings.FieldsFunc(note, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		excl := false
		for _, tok := range strings.Fields(seg) {
			tok = strings.Trim(tok, `()[]{}<>"'`)
			low := strings.ToLower(tok)
			if exclusionWords[low] {
				excl = true
				continue
			}
			e := excl
			if strings.HasPrefix(low, "!") {
				e, low = true, low[1:]
			}
			if e {
				s.Excluded.add(low)
			} else {
				s.rules.add(low)
			}
		}
	}
	return s
}

func (s *rules) add(tok string) {
	switch {
	case tok == "loopback" || tok == "localhost":
		s.Prefixes = append(s.Prefixes, netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128"))
		s.Hosts["localhost"] = true
	case strings.HasPrefix(tok, "*.") && isLabels(tok[2:]):
		s.Domains = append(s.Domains, tok[2:])
	default:
		if p, err := netip.ParsePrefix(tok); err == nil {
			s.Prefixes = append(s.Prefixes, p.Masked())
			return
		}
		h := tok
		if i := strings.Index(h, "://"); i >= 0 {
			h = urlHost(h[i+3:])
		}
		if a, err := netip.ParseAddr(strings.Trim(h, "[]")); err == nil {
			s.Prefixes = append(s.Prefixes, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
		} else if isLabels(h) && strings.Contains(h, ".") {
			s.Hosts[strings.TrimSuffix(h, ".")] = true
		}
	}
}

// overlaps reports whether any of the rules' prefixes intersects lo..hi or a
// name rule matches target — used for exclusions, where touching any excluded
// address inside a range is enough to refuse the whole range.
func (s rules) overlaps(target string) bool {
	if lo, hi, ok := parseRange(target); ok {
		for _, p := range s.Prefixes {
			first := p.Masked().Addr()
			if p.Contains(lo) || (lo.Compare(first) <= 0 && first.Compare(hi) <= 0) {
				return true
			}
		}
		return false
	}
	h := strings.TrimSuffix(target, ".")
	if s.Hosts[h] {
		return true
	}
	for _, d := range s.Domains {
		if h == d || strings.HasSuffix(h, "."+d) {
			return true
		}
	}
	return false
}

// lookup resolves a hostname for IP-only scopes; a var so tests can stub DNS.
var lookup = func(host string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return net.DefaultResolver.LookupHost(ctx, host)
}

// Allows reports whether one target (IP, range "lo-hi", CIDR or hostname) is
// inside the set and not excluded. A hostname no domain rule covers is resolved;
// it is allowed only if every address it resolves to is inside an authorized
// prefix (and none excluded).
func (s Set) Allows(target string) bool {
	if s.Excluded.overlaps(target) {
		return false
	}
	if lo, hi, ok := parseRange(target); ok {
		for _, p := range s.Prefixes {
			if p.Contains(lo) && p.Contains(hi) { // a prefix is contiguous, so all of lo..hi is in it
				return true
			}
		}
		return false
	}
	h := strings.TrimSuffix(target, ".")
	if s.Hosts[h] {
		return true
	}
	for _, d := range s.Domains {
		if h == d || strings.HasSuffix(h, "."+d) {
			return true
		}
	}
	if len(s.Prefixes) == 0 || !strings.Contains(h, ".") {
		return false
	}
	addrs, err := lookup(h)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, raw := range addrs {
		a, err := netip.ParseAddr(raw)
		if err != nil || !s.Allows(a.Unmap().String()) {
			return false
		}
	}
	return true
}

// OutOfScope returns the targets in texts that the set does not allow, in
// first-seen order. texts are argv elements, stdin lines and declared targets.
func (s Set) OutOfScope(texts ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range Targets(texts...) {
		if !seen[t] {
			seen[t] = true
			if !s.Allows(t) {
				out = append(out, t)
			}
		}
	}
	return out
}

var (
	urlRe     = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s'"<>]+`)
	ipRangeRe = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}-\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	// IPv4 with nmap-style octet ranges/wildcards and an optional /prefix.
	ipv4Re = regexp.MustCompile(`\b(?:\d{1,3}|\*)(?:-\d{1,3})?(?:\.(?:\d{1,3}|\*)(?:-\d{1,3})?){3}(?:/\d{1,2})?\b`)
	hostRe = regexp.MustCompile(`[\p{L}\p{N}](?:[\p{L}\p{N}-]*[\p{L}\p{N}])?(?:\.[\p{L}\p{N}](?:[\p{L}\p{N}-]*[\p{L}\p{N}])?)+\.?`)
)

// internalTLDs are non-ICANN suffixes used for internal hosts and AD domains.
var internalTLDs = map[string]bool{"local": true, "lan": true, "corp": true, "internal": true, "intranet": true, "home": true, "localdomain": true, "arpa": true}

// Targets extracts what a command would touch. Absolute and relative file paths
// are skipped; URL hosts that aren't a valid IP or hostname are returned as-is,
// so they can never match scope (fail closed).
func Targets(texts ...string) []string {
	var out []string
	for _, text := range texts {
		for _, tok := range strings.Fields(text) {
			out = append(out, tokenTargets(tok)...)
		}
	}
	return out
}

func tokenTargets(tok string) []string {
	tok = strings.Trim(tok, `"'`)
	if isPath(tok) {
		return nil
	}
	var out []string
	rest := tok
	for _, u := range urlRe.FindAllString(tok, -1) {
		rest = strings.Replace(rest, u, " ", 1)
		out = append(out, normHost(urlHost(u[strings.Index(u, "://")+3:])))
	}
	if strings.HasPrefix(rest, "//") || strings.HasPrefix(rest, `\\`) { // UNC
		h := strings.TrimLeft(rest, `/\`)
		if i := strings.IndexAny(h, `/\`); i >= 0 {
			h = h[:i]
		}
		out = append(out, normHost(urlHost(h)))
		return out
	}
	for _, r := range ipRangeRe.FindAllString(rest, -1) {
		rest = strings.Replace(rest, r, " ", 1)
		out = append(out, r)
	}
	for _, m := range ipv4Re.FindAllString(rest, -1) {
		if ip := normIPv4(m); ip != "" {
			rest = strings.Replace(rest, m, " ", 1)
			out = append(out, ip)
		}
	}
	if h := strings.Trim(stripPort(lastAt(rest)), "[]"); strings.Count(h, ":") >= 2 {
		if a, err := netip.ParseAddr(h); err == nil {
			return append(out, a.Unmap().String())
		}
	}
	for _, h := range hostRe.FindAllString(strings.ToLower(rest), -1) {
		if isHost(h) {
			out = append(out, strings.TrimSuffix(h, "."))
		}
	}
	return out
}

// isPath: absolute, relative or home-relative file paths (not //host UNC).
func isPath(tok string) bool {
	return (strings.HasPrefix(tok, "/") && !strings.HasPrefix(tok, "//")) ||
		strings.HasPrefix(tok, "./") || strings.HasPrefix(tok, "../") || strings.HasPrefix(tok, "~")
}

// urlHost takes "user:pw@host:port/path?q" and returns the host.
func urlHost(s string) string {
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return stripPort(lastAt(s))
}

func lastAt(s string) string {
	if i := strings.LastIndex(s, "@"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func stripPort(s string) string {
	if h, _, err := net.SplitHostPort(s); err == nil {
		return h
	}
	return s
}

// normHost canonicalizes a URL/UNC host: IPs (incl. hex/decimal forms curl
// accepts) to dotted form, names lowercase. Anything else comes back unchanged
// and can't match scope.
func normHost(h string) string {
	h = strings.ToLower(strings.Trim(h, "[]"))
	if a, err := netip.ParseAddr(h); err == nil {
		return a.Unmap().String()
	}
	if ip := inetAton(h); ip != "" {
		return ip
	}
	return strings.TrimSuffix(h, ".")
}

// inetAton handles the numeric host forms inet_aton accepts (decimal, hex,
// octal, 1-4 parts), so http://134744072/ or http://0x08080808/ resolve to 8.8.8.8.
func inetAton(h string) string {
	parts := strings.Split(h, ".")
	if len(parts) > 4 || h == "" {
		return ""
	}
	var nums []uint64
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 0, 32)
		if err != nil {
			return ""
		}
		nums = append(nums, n)
	}
	var v uint64
	switch len(nums) {
	case 1:
		v = nums[0]
	case 2:
		v = nums[0]<<24 | nums[1]
	case 3:
		v = nums[0]<<24 | nums[1]<<16 | nums[2]
	case 4:
		v = nums[0]<<24 | nums[1]<<16 | nums[2]<<8 | nums[3]
	}
	if v > 0xffffffff {
		return ""
	}
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}).String()
}

// normIPv4 turns an IPv4 match into an address, a CIDR, or a "lo-hi" range
// (nmap octet ranges like 10.0.0.1-254 and 10.0.*.1).
func normIPv4(m string) string {
	if p, err := netip.ParsePrefix(m); err == nil {
		return p.Masked().String()
	}
	if a, err := netip.ParseAddr(m); err == nil {
		return a.String()
	}
	var lo, hi [4]byte
	for i, oct := range strings.Split(m, ".") {
		a, b, isRange := strings.Cut(oct, "-")
		switch {
		case oct == "*":
			lo[i], hi[i] = 0, 255
		case isRange:
			x, e1 := strconv.Atoi(a)
			y, e2 := strconv.Atoi(b)
			if e1 != nil || e2 != nil || x > 255 || y > 255 {
				return ""
			}
			lo[i], hi[i] = byte(min(x, y)), byte(max(x, y))
		default:
			x, err := strconv.Atoi(oct)
			if err != nil || x > 255 {
				return ""
			}
			lo[i], hi[i] = byte(x), byte(x)
		}
	}
	return netip.AddrFrom4(lo).String() + "-" + netip.AddrFrom4(hi).String()
}

func parseRange(t string) (lo, hi netip.Addr, ok bool) {
	if p, err := netip.ParsePrefix(t); err == nil {
		lo = p.Masked().Addr()
		hi = lo
		for n := p.Bits(); n < lo.BitLen(); n++ { // set the host bits
			b := hi.AsSlice()
			b[n/8] |= 1 << (7 - n%8)
			hi, _ = netip.AddrFromSlice(b)
		}
		return lo, hi, true
	}
	if a, b, found := strings.Cut(t, "-"); found {
		x, e1 := netip.ParseAddr(a)
		y, e2 := netip.ParseAddr(b)
		if e1 == nil && e2 == nil {
			return x, y, true
		}
	}
	if a, err := netip.ParseAddr(t); err == nil {
		return a.Unmap(), a.Unmap(), true
	}
	return netip.Addr{}, netip.Addr{}, false
}

// isLabels: dot-separated letters/digits/hyphens (any script).
func isLabels(h string) bool {
	h = strings.TrimSuffix(h, ".")
	if h == "" || len(h) > 253 {
		return false
	}
	for _, l := range strings.Split(h, ".") {
		if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return false
		}
		for _, r := range l {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' {
				return false
			}
		}
	}
	return true
}

// isHost: a dotted name whose TLD is a real ICANN TLD, a common internal TLD,
// or non-ASCII (IDN). Keeps filenames like rules.yar and web.config out.
func isHost(h string) bool {
	h = strings.TrimSuffix(h, ".")
	if !strings.Contains(h, ".") || !isLabels(h) {
		return false
	}
	tld := h[strings.LastIndex(h, ".")+1:]
	if strings.IndexFunc(tld, func(r rune) bool { return r > unicode.MaxASCII }) >= 0 {
		return true
	}
	if internalTLDs[tld] {
		return true
	}
	if _, icann := publicsuffix.PublicSuffix(tld); icann && strings.IndexFunc(tld, unicode.IsDigit) < 0 {
		return true
	}
	return false
}

// ListFlags read target lists from a file (nmap/masscan -iL, nuclei/httpx/naabu/
// dnsx -l or -list, subfinder -dL, hydra -M). Their files' contents are
// scope-checked too; a value that isn't an existing file (hydra -l <login>) is
// left to the normal token scan.
var ListFlags = []string{"-iL", "-l", "-list", "--list", "-dL", "-M"}

// FileTargets reads a target-list file (≤1 MiB) and returns its targets.
func FileTargets(path string) ([]string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() > 1<<20 {
		return nil, os.ErrInvalid
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Targets(string(b)), nil
}

// Entries lists the set's authorized targets in their scope-note form.
func (s Set) Entries() []string {
	var out []string
	for _, p := range s.Prefixes {
		out = append(out, p.String())
	}
	for h := range s.Hosts {
		out = append(out, h)
	}
	for _, d := range s.Domains {
		out = append(out, "*."+d)
	}
	return out
}

// Added returns the entries of newNote that oldNote doesn't already list —
// what a set_engagement call would newly authorize.
func Added(oldNote, newNote string) []string {
	have := map[string]bool{}
	for _, e := range Parse(oldNote).Entries() {
		have[e] = true
	}
	var out []string
	for _, e := range Parse(newNote).Entries() {
		if !have[e] {
			out = append(out, e)
		}
	}
	slices.Sort(out)
	return out
}
