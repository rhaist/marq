package registry

import "github.com/rhaist/marq/internal/files"

// fileTools returns the sandboxed file-access tools (in-process Handler tools,
// confined to /work and /tmp by the files package).
func fileTools() []Tool {
	return []Tool{
		{
			Name: "list_dir",
			Desc: "List a directory inside the working area (/work or /tmp). Shows entry names, type " +
				"(dir/file) and size. Use this to find files other tools wrote, or to confirm a staged input.",
			Params: []Param{
				{Name: "path", Type: StringParam, Desc: "directory inside /work or /tmp", Default: "/work"},
			},
			Handler: func(a Args) string { return files.ListDir(a.S("path")) },
		},
		{
			Name: "read_file",
			Desc: "Read a text file from the working area (/work or /tmp). Returns the contents (truncated " +
				"to the server output budget, or to `max_bytes` if smaller). Use it to retrieve results " +
				"tools wrote to disk.",
			Params: []Param{
				{Name: "path", Type: StringParam, Desc: "file inside /work or /tmp", Required: true},
				{Name: "max_bytes", Type: IntParam, Desc: "max bytes to read (0 = budget)", Default: 0},
			},
			Handler: func(a Args) string { return files.ReadFile(a.S("path"), a.I("max_bytes")) },
		},
		{
			Name: "write_file",
			Desc: "Write a text file into the working area (/work or /tmp), creating parent dirs as needed. " +
				"Use it to stage inputs for other tools — e.g. a captured hash for john, a target list for " +
				"httpx_probe/dnsx, or a custom wordlist. Overwrites an existing file at `path`.",
			Params: []Param{
				{Name: "path", Type: StringParam, Desc: "file inside /work or /tmp", Required: true},
				{Name: "content", Type: StringParam, Desc: "text content to write", Required: true},
			},
			Handler: func(a Args) string { return files.WriteFile(a.S("path"), a.S("content")) },
		},
	}
}
