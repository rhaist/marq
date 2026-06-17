package findings

import "testing"

func TestCVSSBase(t *testing.T) {
	cases := []struct {
		vector   string
		score    float64
		severity string
		ok       bool
	}{
		// Canonical worst-case (CVSS 3.1 reference) = 9.8 Critical.
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", 9.8, "critical", true},
		// Scope-changed example = 10.0 Critical.
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H", 10.0, "critical", true},
		// A medium and a low.
		{"CVSS:3.1/AV:N/AC:L/PR:N/UI:R/S:U/C:L/I:N/A:N", 4.3, "medium", true},
		{"CVSS:3.1/AV:N/AC:H/PR:L/UI:R/S:U/C:L/I:L/A:N", 3.7, "low", true},
		// Missing metrics -> not ok.
		{"CVSS:3.1/AV:N/AC:L", 0, "", false},
		{"garbage", 0, "", false},
	}
	for _, c := range cases {
		score, sev, ok := cvssBase(c.vector)
		if ok != c.ok {
			t.Errorf("%s: ok=%v want %v", c.vector, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if score != c.score || sev != c.severity {
			t.Errorf("%s: got %.1f/%s want %.1f/%s", c.vector, score, sev, c.score, c.severity)
		}
	}
}
