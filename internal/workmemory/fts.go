package workmemory

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type ftsHit struct {
	ID      string
	Rank    float64
	Snippet string
}

type ftsIndex struct {
	mu   sync.Mutex
	path string
	conn *sqlite.Conn
}

func openFTSIndex(path string) (*ftsIndex, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if err := ensureParentDir(path); err != nil {
		return nil, err
	}
	conn, err := sqlite.OpenConn(path, sqlite.OpenReadWrite, sqlite.OpenCreate, sqlite.OpenWAL)
	if err != nil {
		return nil, err
	}
	conn.SetBlockOnBusy()
	index := &ftsIndex{path: path, conn: conn}
	if err := index.migrate(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return index, nil
}

func (idx *ftsIndex) Path() string {
	if idx == nil {
		return ""
	}
	return idx.path
}

func (idx *ftsIndex) Close() {
	if idx == nil {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.conn != nil {
		_ = idx.conn.Close()
		idx.conn = nil
	}
}

func (idx *ftsIndex) migrate() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.conn == nil {
		return nil
	}
	return sqlitex.ExecScript(idx.conn, `
CREATE VIRTUAL TABLE IF NOT EXISTS work_memory_fts USING fts5(
  id UNINDEXED,
  title,
  summary,
  body,
  ocr,
  window_title,
  app_name,
  tags,
  tokenize = 'unicode61 remove_diacritics 2'
);
`)
}

func (idx *ftsIndex) Rebuild(entries []Entry) error {
	if idx == nil || idx.conn == nil {
		return nil
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	var err error
	end, err := sqlitex.ImmediateTransaction(idx.conn)
	if err != nil {
		return err
	}
	defer end(&err)
	if err = sqlitex.Execute(idx.conn, `DELETE FROM work_memory_fts;`, nil); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.ID == "" {
			continue
		}
		if !entryUsableForExtraction(entry) {
			continue
		}
		if err = sqlitex.Execute(idx.conn, `INSERT INTO work_memory_fts(id, title, summary, body, ocr, window_title, app_name, tags) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8);`, &sqlitex.ExecOptions{
			Args: []any{
				entry.ID,
				entry.Title,
				entry.Summary,
				entry.Text,
				entry.OCRText,
				entry.WindowTitle,
				entry.AppName,
				strings.Join(entry.Tags, " "),
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (idx *ftsIndex) Search(query string, limit int) ([]ftsHit, error) {
	if idx == nil || idx.conn == nil {
		return nil, nil
	}
	match := ftsMatchQuery(query)
	if match == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	hits := []ftsHit{}
	err := sqlitex.Execute(idx.conn, `SELECT id, bm25(work_memory_fts) AS rank, snippet(work_memory_fts, -1, '', '', ' ... ', 16) AS snippet_text FROM work_memory_fts WHERE work_memory_fts MATCH ?1 ORDER BY rank LIMIT ?2;`, &sqlitex.ExecOptions{
		Args: []any{match, limit},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			hits = append(hits, ftsHit{
				ID:      stmt.ColumnText(0),
				Rank:    stmt.ColumnFloat(1),
				Snippet: strings.TrimSpace(stmt.ColumnText(2)),
			})
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return hits, nil
}

func (idx *ftsIndex) Count() (int, error) {
	if idx == nil || idx.conn == nil {
		return 0, nil
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	count := 0
	err := sqlitex.Execute(idx.conn, `SELECT count(*) FROM work_memory_fts;`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			count = stmt.ColumnInt(0)
			return nil
		},
	})
	return count, err
}

func ftsPathForMemoryPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	ext := filepath.Ext(path)
	if ext == "" {
		return path + ".fts.sqlite"
	}
	return strings.TrimSuffix(path, ext) + ".fts.sqlite"
}

func ftsMatchQuery(query string) string {
	tokens := ftsTokens(query)
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if isASCIIFTSToken(token) {
			parts = append(parts, token+"*")
			continue
		}
		parts = append(parts, `"`+strings.ReplaceAll(token, `"`, `""`)+`"`)
	}
	return strings.Join(parts, " AND ")
}

func ftsTokens(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	tokens := []string{}
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
	}
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func isASCIIFTSToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r > unicode.MaxASCII {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_') {
			return false
		}
	}
	return true
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
