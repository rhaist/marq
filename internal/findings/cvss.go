package findings

import (
	"math"
	"strings"
)

// cvssBase computes the CVSS 3.1 base score and severity rating from a vector
// string (e.g. "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"). ok is false if
// the vector is missing required metrics. Pure arithmetic — no LLM, no network.
func cvssBase(vector string) (score float64, severity string, ok bool) {
	m := map[string]string{}
	for _, part := range strings.Split(vector, "/") {
		if k, v, found := strings.Cut(part, ":"); found {
			m[strings.ToUpper(strings.TrimSpace(k))] = strings.ToUpper(strings.TrimSpace(v))
		}
	}

	av, ok1 := map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2}[m["AV"]]
	ac, ok2 := map[string]float64{"L": 0.77, "H": 0.44}[m["AC"]]
	ui, ok3 := map[string]float64{"N": 0.85, "R": 0.62}[m["UI"]]
	scope := m["S"]
	prTable := map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	if scope == "C" {
		prTable = map[string]float64{"N": 0.85, "L": 0.68, "H": 0.50}
	}
	pr, ok4 := prTable[m["PR"]]
	cia := map[string]float64{"H": 0.56, "L": 0.22, "N": 0}
	c, ok5 := cia[m["C"]]
	i, ok6 := cia[m["I"]]
	a, ok7 := cia[m["A"]]

	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7) || (scope != "U" && scope != "C") {
		return 0, "", false
	}

	iss := 1 - (1-c)*(1-i)*(1-a)
	var impact float64
	if scope == "C" {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	} else {
		impact = 6.42 * iss
	}
	if impact <= 0 {
		return 0, "none", true
	}
	expl := 8.22 * av * ac * pr * ui
	sum := impact + expl
	if scope == "C" {
		sum *= 1.08
	}
	base := roundup(math.Min(sum, 10))
	return base, rating(base), true
}

func roundup(x float64) float64 { return math.Ceil(x*10) / 10 }

// rating maps a base score to the CVSS qualitative severity.
func rating(s float64) string {
	switch {
	case s <= 0:
		return "none"
	case s < 4:
		return "low"
	case s < 7:
		return "medium"
	case s < 9:
		return "high"
	default:
		return "critical"
	}
}
