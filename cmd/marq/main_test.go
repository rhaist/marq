package main

import "testing"

func TestParseToolArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want map[string]string // expected key->string value (ints/bools compared as strings)
		err  bool
	}{
		{"json", []string{`{"target":"x"}`}, map[string]string{"target": "x"}, false},
		{"flags", []string{"--target", "x", "--port", "22"}, map[string]string{"target": "x", "port": "22"}, false},
		{"equals", []string{"--target=x"}, map[string]string{"target": "x"}, false},
		{"json-flag", []string{"--json", `{"a":"b"}`}, map[string]string{"a": "b"}, false},
		{"bare-flag", []string{"--deep"}, map[string]string{"deep": "true"}, false},
		{"bad-json", []string{"{nope"}, nil, true},
	}
	for _, c := range cases {
		got, err := parseToolArgs(c.in)
		if (err != nil) != c.err {
			t.Errorf("%s: err=%v want err=%v", c.name, err, c.err)
			continue
		}
		if c.err {
			continue
		}
		for k, v := range c.want {
			if gv, ok := got[k].(string); !ok || gv != v {
				t.Errorf("%s: key %q = %v, want %q", c.name, k, got[k], v)
			}
		}
	}
}
