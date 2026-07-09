package registry

import (
	"strconv"

	"github.com/rhaist/marq/internal/shellword"
)

// creds returns the offline password/hash cracking tools.
func creds() []Tool {
	return []Tool{
		{
			Name: "john",
			Desc: "Crack hashes in a local file with John the Ripper. Provide extra flags (e.g. --format=, " +
				"--wordlist=) via `options`. `hash_file` must be a path inside the container.",
			Params: []Param{
				{Name: "hash_file", Type: StringParam, Desc: "hash file path inside the container", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra john flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"john"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("hash_file"))
				return Invocation{Argv: argv, Target: a.S("hash_file")}
			},
		},
		{
			Name: "hashcat",
			Desc: "Crack hashes with hashcat. `mode` is the -m hash type, `wordlist` an attack dictionary " +
				"path inside the container. A CPU OpenCL runtime is bundled so it runs without a GPU — fine " +
				"for weak/fast hashes; real cracking needs '--gpus all' at docker run. '--force' is added so " +
				"the CPU device is accepted; pass extra flags via `options`.",
			Params: []Param{
				{Name: "hash_file", Type: StringParam, Desc: "hash file path", Required: true},
				{Name: "mode", Type: IntParam, Desc: "hashcat -m hash type", Required: true},
				{Name: "wordlist", Type: StringParam, Desc: "wordlist path", Required: true},
				{Name: "options", Type: StringParam, Desc: "extra hashcat flags", Default: ""},
			},
			Build: func(a Args) Invocation {
				argv := []string{"hashcat", "-m", strconv.Itoa(a.I("mode")), "--force"}
				argv = append(argv, shellword.Split(a.S("options"))...)
				argv = append(argv, a.S("hash_file"), a.S("wordlist"))
				return Invocation{Argv: argv, Target: a.S("hash_file")}
			},
		},
		{
			Name: "hash_identify",
			Desc: "Identify the likely type of a hash string with name-that-hash (nth), ranked " +
				"most-likely first with the matching hashcat/john modes. Feed the top mode into hashcat.",
			Params: []Param{
				{Name: "hash_value", Type: StringParam, Desc: "hash string", Required: true},
			},
			Build: func(a Args) Invocation {
				// name-that-hash: -t takes the hash string directly; it prints ranked
				// candidates (most-likely first) by default. --no-banner keeps the
				// output clean for the model.
				return Invocation{Argv: []string{"nth", "--no-banner", "-t", a.S("hash_value")}, Target: "(hash)"}
			},
		},
	}
}
