package laws

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rhaist/marq/internal/runner"
)

// maxBody caps one source download; the longest consolidated acts are a few MB.
const maxBody = 64 << 20

// Result is the outcome of syncing one source.
type Result struct {
	ID       string
	Articles int
	Err      error
}

// Sync fetches each source (all of them, or only the ids listed) from its
// official URL and stores it in the corpus at path. A source that fails keeps
// its previously synced version. Sources are fetched one at a time, so
// government sites see one polite client rather than a burst.
func Sync(ctx context.Context, path string, sources []Source, only []string, client *http.Client) ([]Result, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, s := range sources {
		known[s.ID] = true
	}
	for _, id := range only {
		if !known[id] {
			return nil, fmt.Errorf("unknown law id %q (see internal/laws/sources/*.json)", id)
		}
	}
	st, err := openRW(path)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	var out []Result
	for _, src := range sources {
		if len(only) > 0 && !slices.Contains(only, src.ID) {
			continue
		}
		n, err := syncOne(ctx, st, src, client)
		out = append(out, Result{src.ID, n, err})
	}
	return out, nil
}

func syncOne(ctx context.Context, st *Store, src Source, client *http.Client) (int, error) {
	text, err := fetch(ctx, src, client)
	if err != nil {
		return 0, err
	}
	if src.Start != "" {
		i := strings.Index(text, src.Start)
		if i < 0 {
			return 0, fmt.Errorf("start marker %q not found", src.Start)
		}
		text = text[i:]
	}
	arts := Split(text, src.Lang, src.Split, src.MaxGap)
	// a redirect to a home page or an error page also lands here
	if len(arts) == 1 && arts[0].Num == "all" {
		return 0, fmt.Errorf("no article headings found in %d chars (a redirect or error page?); check the URL or add a split regex", len(text))
	}
	return len(arts), st.put(src, arts, time.Now())
}

func fetch(ctx context.Context, src Source, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return "", err
	}
	// A generic client name: no operator details leave the box.
	req.Header.Set("User-Agent", "marq-laws-sync/1 (+https://github.com/rhaist/marq)")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxBody {
		return "", fmt.Errorf("larger than %d MB", maxBody>>20)
	}
	var text string
	switch src.Format {
	case "pdf":
		text, err = pdfToText(body, src.URL)
	default:
		if text, err = decode(body, resp.Header.Get("Content-Type")); err == nil && src.Format == "html" {
			text = HTMLToText(text)
		}
	}
	if err != nil {
		return "", err
	}
	text = tidy(text)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("page has no text (a script-rendered page?)")
	}
	return text, nil
}

// pdfToText converts a PDF gazette with poppler's pdftotext, through the audited
// runner like every other external binary. pdftotext writes to a file because
// the runner caps stdout at the model's output budget, and a law is longer.
// The Docker image ships pdftotext; elsewhere, install poppler-utils.
func pdfToText(body []byte, url string) (string, error) {
	dir, err := os.MkdirTemp("", "marq-pdf-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	in, out := filepath.Join(dir, "in.pdf"), filepath.Join(dir, "out.txt")
	if err := os.WriteFile(in, body, 0o600); err != nil {
		return "", err
	}
	res := runner.Run("laws_pdftotext", []string{"pdftotext", "-enc", "UTF-8", "-nopgbrk", in, out}, runner.Opts{Target: url})
	if res.ExitCode == nil || *res.ExitCode != 0 {
		return "", fmt.Errorf("pdftotext failed (PDF sources need poppler-utils; the Docker image has it): %s", strings.TrimSpace(res.Stderr))
	}
	b, err := os.ReadFile(out)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("pdftotext produced invalid UTF-8")
	}
	return string(b), nil
}
