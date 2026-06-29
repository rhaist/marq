package registry

import "testing"

func TestParseImpacketTarget(t *testing.T) {
	cases := []struct{ in, d, u, p, h string }{
		{"corp/alice:s3cr3t@dc01", "corp", "alice", "s3cr3t", "dc01"},
		{"alice:s3cr3t@dc01", "", "alice", "s3cr3t", "dc01"},
		{"corp/alice@dc01", "corp", "alice", "", "dc01"}, // no password — must not panic
		{"alice", "", "alice", "", ""},                   // bare user — the old certipy parse panicked here
		{"corp/alice:p@ss@dc01", "corp", "alice", "p@ss", "dc01"}, // @ in password, host is last segment
	}
	for _, c := range cases {
		d, u, p, h := parseImpacketTarget(c.in)
		if d != c.d || u != c.u || p != c.p || h != c.h {
			t.Errorf("%q -> (%q,%q,%q,%q), want (%q,%q,%q,%q)", c.in, d, u, p, h, c.d, c.u, c.p, c.h)
		}
	}
}
