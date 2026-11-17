package laws

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite" // pure-Go SQLite with FTS5, keeps the binary CGO-free
)

// shortcut: one SQLite file with an FTS5 trigram index (substring search in any
// script); fine for a few hundred MB of law text. Move to bleve or Tantivy only
// if ranking quality, not lookup, becomes the problem.
const schema = `
CREATE TABLE IF NOT EXISTS laws(id TEXT PRIMARY KEY, title TEXT, native_title TEXT, jurisdiction TEXT,
  lang TEXT, kind TEXT, translates TEXT, url TEXT, version_date TEXT, fetched_at TEXT, note TEXT);
CREATE TABLE IF NOT EXISTS terms(law_id TEXT, term TEXT);
CREATE INDEX IF NOT EXISTS terms_term ON terms(term);
CREATE TABLE IF NOT EXISTS articles(id INTEGER PRIMARY KEY, law_id TEXT, pos INTEGER, num TEXT, heading TEXT, body TEXT);
CREATE INDEX IF NOT EXISTS articles_law ON articles(law_id, num);
CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(norm, tokenize='trigram');`

// ErrNoCorpus means the corpus file doesn't exist yet.
var ErrNoCorpus = errors.New("law corpus not installed")

// Store is an open corpus file.
type Store struct{ db *sql.DB }

// Open opens the corpus read-only; it never creates the file.
func Open(path string) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, ErrNoCorpus
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	return &Store{db}, nil
}

// openRW opens (creating if needed) the corpus for sync.
func openRW(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}

// Close releases the file.
func (s *Store) Close() error { return s.db.Close() }

// put replaces one law's rows in a single transaction, so a failed sync of a
// source leaves its previous version intact.
func (s *Store) put(src Source, arts []Article, fetched time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM articles_fts WHERE rowid IN (SELECT id FROM articles WHERE law_id = ?)`,
		`DELETE FROM articles WHERE law_id = ?`, `DELETE FROM terms WHERE law_id = ?`, `DELETE FROM laws WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, src.ID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO laws VALUES(?,?,?,?,?,?,?,?,?,?,?)`, src.ID, src.Title, src.NativeTitle,
		src.Jurisdiction, src.Lang, src.Kind, src.Translates, src.URL, src.VersionDate, fetched.UTC().Format(time.DateOnly), src.Note); err != nil {
		return err
	}
	for _, t := range append([]string{src.ID, src.Title, src.NativeTitle}, src.Terms...) {
		if t = norm(t); t != "" {
			if _, err := tx.Exec(`INSERT INTO terms VALUES(?,?)`, src.ID, t); err != nil {
				return err
			}
		}
	}
	for i, a := range arts {
		res, err := tx.Exec(`INSERT INTO articles(law_id,pos,num,heading,body) VALUES(?,?,?,?,?)`, src.ID, i, a.Num, a.Heading, a.Body)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		if _, err := tx.Exec(`INSERT INTO articles_fts(rowid, norm) VALUES(?,?)`, id, norm(a.Body)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// norm is applied to both indexed text and queries: lower-case, Arabic vowel
// marks and letter variants unified.
func norm(s string) string { return strings.ToLower(normArabic(strings.TrimSpace(s))) }

// Law is one corpus entry's metadata.
type Law struct {
	ID, Title, NativeTitle, Jurisdiction, Lang, Kind, Translates, URL, VersionDate, FetchedAt, Note string
	Articles                                                                                        int
}

const lawCols = `l.id,l.title,l.native_title,l.jurisdiction,l.lang,l.kind,l.translates,l.url,l.version_date,l.fetched_at,l.note,
  (SELECT count(*) FROM articles a WHERE a.law_id = l.id)`

func scanLaw(r interface{ Scan(...any) error }) (Law, error) {
	var l Law
	err := r.Scan(&l.ID, &l.Title, &l.NativeTitle, &l.Jurisdiction, &l.Lang, &l.Kind, &l.Translates, &l.URL,
		&l.VersionDate, &l.FetchedAt, &l.Note, &l.Articles)
	return l, err
}

// List returns every law in the corpus, by jurisdiction then id.
func (s *Store) List() ([]Law, error) {
	rows, err := s.db.Query(`SELECT ` + lawCols + ` FROM laws l ORDER BY l.jurisdiction, l.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Law
	for rows.Next() {
		l, err := scanLaw(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Resolve finds a law by id, title or key term ("PIPL", "KVKK", "个人信息保护法").
func (s *Store) Resolve(ref string) (Law, error) {
	l, err := scanLaw(s.db.QueryRow(`SELECT `+lawCols+` FROM laws l WHERE l.id = ?
	  OR l.id = (SELECT law_id FROM terms WHERE term = ? LIMIT 1)`, ref, norm(ref)))
	if errors.Is(err, sql.ErrNoRows) {
		return l, fmt.Errorf("no law %q in the corpus (call law_lookup with no arguments to list them)", ref)
	}
	return l, err
}

// Article returns one article of a law; num may be in any supported style.
func (s *Store) Article(lawID, num string) (Article, error) {
	var a Article
	err := s.db.QueryRow(`SELECT num, heading, body FROM articles WHERE law_id = ? AND num = ? ORDER BY pos LIMIT 1`,
		lawID, NormalizeNum(num)).Scan(&a.Num, &a.Heading, &a.Body)
	if errors.Is(err, sql.ErrNoRows) {
		return a, fmt.Errorf("article %q not found in %s", num, lawID)
	}
	return a, err
}

// Nums lists a law's article numbers in order.
func (s *Store) Nums(lawID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT num FROM articles WHERE law_id = ? ORDER BY pos`, lawID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Hit is one search result.
type Hit struct {
	LawID, Num, Snippet string
}

// Search finds articles containing every word of query (substring match in any
// script), optionally within one law. Words of 3+ characters use the trigram
// index; shorter ones (common in Chinese and Japanese) fall back to LIKE.
func (s *Store) Search(query, lawID string, limit int) ([]Hit, error) {
	var long, short []string
	for _, w := range strings.Fields(norm(query)) {
		if utf8.RuneCountInString(w) >= 3 {
			long = append(long, `"`+strings.ReplaceAll(w, `"`, `""`)+`"`)
		} else {
			short = append(short, w)
		}
	}
	if len(long)+len(short) == 0 {
		return nil, errors.New("empty query")
	}
	q := `SELECT a.law_id, a.num, f.norm FROM articles_fts f JOIN articles a ON a.id = f.rowid WHERE 1=1`
	var args []any
	if len(long) > 0 {
		q += ` AND articles_fts MATCH ?`
		args = append(args, strings.Join(long, " AND "))
	}
	for _, w := range short {
		q += ` AND f.norm LIKE ? ESCAPE '\'`
		args = append(args, "%"+strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(w)+"%")
	}
	if lawID != "" {
		q += ` AND a.law_id = ?`
		args = append(args, lawID)
	}
	if len(long) > 0 {
		q += ` ORDER BY bm25(articles_fts)`
	} else {
		q += ` ORDER BY a.law_id, a.pos`
	}
	q += ` LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	first := strings.Trim(append(long, short...)[0], `"`)
	var out []Hit
	for rows.Next() {
		var h Hit
		var text string
		if err := rows.Scan(&h.LawID, &h.Num, &text); err != nil {
			return nil, err
		}
		h.Snippet = snippet(text, first, 90)
		out = append(out, h)
	}
	return out, rows.Err()
}

// snippet returns about width runes either side of the first match of word.
func snippet(text, word string, width int) string {
	r := []rune(text)
	i := strings.Index(text, word)
	if i < 0 {
		return string(r[:min(len(r), 2*width)])
	}
	at := utf8.RuneCountInString(text[:i])
	from, to := max(0, at-width), min(len(r), at+utf8.RuneCountInString(word)+width)
	out := strings.ReplaceAll(string(r[from:to]), "\n", " ")
	if from > 0 {
		out = "…" + out
	}
	if to < len(r) {
		out += "…"
	}
	return out
}

// Stale reports whether a reference translation predates the version of the
// official text it translates, and if so a warning for the reader.
func (s *Store) Stale(l Law) string {
	if l.Kind != "reference" || l.Translates == "" {
		return ""
	}
	o, err := s.Resolve(l.Translates)
	if err != nil || o.VersionDate <= l.VersionDate {
		return ""
	}
	return fmt.Sprintf("WARNING: this translation is dated %s but the official text (%s) was amended %s; check the original.",
		l.VersionDate, o.ID, o.VersionDate)
}
