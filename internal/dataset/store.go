package dataset

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 5

type Store struct {
	db      *sql.DB
	root    string
	writeMu sync.Mutex
}
type Title struct {
	ID, Name, URL, AlternativeTitle, OriginalLanguage, Status, ContentRating, Stratum, Split, State, ProviderCreatedAt, ProviderUpdatedAt string
	Tags, AvailableLanguages                                                                                                              []string
	Year, DiscoveryOrder, SampleRank                                                                                                      int
}
type Chapter struct {
	ID, TitleID, Number, Name, Volume, Language, ReleaseGroup, PublishedAt, URL, State, OutputPath string
	ExpectedPages, ProviderOrder                                                                   int
}
type Page struct {
	TitleID, ChapterID, RelativePath, ArchiveEntry, StorageType, SourceMIMEType, MIMEType, Extension, SHA256, PerceptualHash, State, RejectionCode, ErrorMessage, ExactDuplicateOf, NearDuplicateOf, Split string
	Index, Width, Height                                                                                                                                                                                   int
	Bytes                                                                                                                                                                                                  int64
}
type Counters struct {
	DiscoveredTitles  int   `json:"discoveredTitles"`
	PlannedTitles     int   `json:"plannedTitles"`
	CompletedTitles   int   `json:"completedTitles"`
	FailedTitles      int   `json:"failedTitles"`
	PlannedChapters   int   `json:"plannedChapters"`
	CompletedChapters int   `json:"completedChapters"`
	FailedChapters    int   `json:"failedChapters"`
	PlannedPages      int   `json:"plannedPages"`
	ValidPages        int   `json:"validPages"`
	DuplicatePages    int   `json:"duplicatePages"`
	RejectedPages     int   `json:"rejectedPages"`
	FailedPages       int   `json:"failedPages"`
	Archives          int   `json:"archives"`
	StoredBytes       int64 `json:"storedBytes"`
}
type DatasetInfo struct {
	Config         Config         `json:"config"`
	ConfigHash     string         `json:"configurationHash"`
	State          string         `json:"state"`
	StoppingReason string         `json:"stoppingReason"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
	Counters       Counters       `json:"counters"`
	SplitCounts    map[string]int `json:"splitCounts"`
}

func Open(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create dataset root: %w", err)
	}
	return open(root)
}

// OpenExisting opens a previously initialized dataset without creating its
// root or database. Read-oriented commands use it so a misspelled path does
// not leave a surprising empty SQLite file behind.
func OpenExisting(root string) (*Store, error) {
	path := filepath.Join(root, "dataset.sqlite")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("dataset not found at %q", root)
		}
		return nil, fmt.Errorf("inspect dataset database: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("dataset database is a directory at %q", path)
	}
	return open(root)
}

func open(root string) (*Store, error) {
	path := filepath.Join(root, "dataset.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open dataset database: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure dataset database: %w", err)
	}
	s := &Store{db: db, root: root}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Root() string { return s.root }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS dataset_meta (id INTEGER PRIMARY KEY CHECK(id=1), schema_version INTEGER NOT NULL, config_json TEXT NOT NULL, config_hash TEXT NOT NULL, state TEXT NOT NULL, stopping_reason TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL, completed_at TEXT, final_error TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS titles (id TEXT PRIMARY KEY, name TEXT NOT NULL, url TEXT, alternative_title TEXT, original_language TEXT, status TEXT, content_rating TEXT, year INTEGER, discovery_order INTEGER NOT NULL, stratum TEXT, sample_rank INTEGER, split TEXT, state TEXT NOT NULL, planned_chapters INTEGER NOT NULL DEFAULT 0, completed_chapters INTEGER NOT NULL DEFAULT 0, failure_count INTEGER NOT NULL DEFAULT 0, last_error TEXT, provider_created_at TEXT, provider_updated_at TEXT, tags_json TEXT, available_languages_json TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS chapters (id TEXT PRIMARY KEY, title_id TEXT NOT NULL REFERENCES titles(id), number TEXT, name TEXT, volume TEXT, language TEXT, release_group TEXT, published_at TEXT, url TEXT, provider_order INTEGER NOT NULL, expected_pages INTEGER NOT NULL DEFAULT 0, selected INTEGER NOT NULL, output_path TEXT, archive_path TEXT, state TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT, claim_owner TEXT, claimed_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS pages (title_id TEXT NOT NULL, chapter_id TEXT NOT NULL REFERENCES chapters(id), page_index INTEGER NOT NULL, storage_type TEXT, relative_path TEXT, archive_entry TEXT, source_mime_type TEXT, mime_type TEXT, extension TEXT, width INTEGER, height INTEGER, bytes INTEGER, sha256 TEXT, perceptual_hash TEXT, exact_duplicate_of TEXT, near_duplicate_of TEXT, state TEXT NOT NULL, rejection_code TEXT, attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT, downloaded_at TEXT, validated_at TEXT, PRIMARY KEY(chapter_id, page_index));
CREATE TABLE IF NOT EXISTS attempts (id INTEGER PRIMARY KEY AUTOINCREMENT, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL, operation TEXT NOT NULL, attempt INTEGER NOT NULL, retryable INTEGER NOT NULL, message TEXT, created_at TEXT NOT NULL);`)
	if err != nil {
		return fmt.Errorf("migrate dataset database: %w", err)
	}
	for _, column := range []struct{ table, definition string }{{"chapters", "claim_owner TEXT"}, {"chapters", "claimed_at TEXT"}, {"pages", "near_duplicate_of TEXT"}, {"pages", "source_mime_type TEXT"}, {"pages", "extension TEXT"}, {"titles", "alternative_title TEXT"}, {"titles", "provider_created_at TEXT"}, {"titles", "provider_updated_at TEXT"}, {"titles", "tags_json TEXT"}, {"titles", "available_languages_json TEXT"}} {
		if err := s.ensureColumn(ctx, column.table, column.definition); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, "UPDATE dataset_meta SET schema_version=? WHERE schema_version<?", SchemaVersion, SchemaVersion)
	return err
}

func (s *Store) ensureColumn(ctx context.Context, table, definition string) error {
	name := strings.Fields(definition)[0]
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var column, kind string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &column, &kind, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if column == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+definition); err != nil {
		return fmt.Errorf("migrate dataset database: add %s.%s: %w", table, name, err)
	}
	return nil
}

func (s *Store) LoadConfig(ctx context.Context) (Config, string, bool, error) {
	var data, hash string
	err := s.db.QueryRowContext(ctx, "SELECT config_json, config_hash FROM dataset_meta WHERE id=1").Scan(&data, &hash)
	if err == sql.ErrNoRows {
		return Config{}, "", false, nil
	}
	if err != nil {
		return Config{}, "", false, err
	}
	var cfg Config
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		return Config{}, "", false, err
	}
	return cfg, hash, true, nil
}
func (s *Store) Initialize(ctx context.Context, cfg Config, resume bool) error {
	if err := cfg.Normalize(); err != nil {
		return err
	}
	data, err := cfg.CanonicalJSON()
	if err != nil {
		return err
	}
	hash, err := cfg.Hash()
	if err != nil {
		return err
	}
	existing, existingHash, exists, err := s.LoadConfig(ctx)
	if err != nil {
		return err
	}
	if exists {
		if existingHash != hash {
			return fmt.Errorf("dataset configuration mismatch: existing format %s and requested format %s; resume with the saved configuration or create a new dataset", existing.Output.Format, cfg.Output.Format)
		}
		if !resume {
			return fmt.Errorf("dataset already exists; use --resume to continue it")
		}
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, "INSERT INTO dataset_meta(id,schema_version,config_json,config_hash,state,created_at,updated_at) VALUES(1,?,?,?,?,?,?)", SchemaVersion, string(data), hash, "new", now, now)
	if err != nil {
		return fmt.Errorf("initialize dataset database: %w", err)
	}
	if err := writeJSONFile(filepath.Join(s.root, "dataset-config.json"), cfg); err != nil {
		return fmt.Errorf("persist dataset configuration: %w", err)
	}
	return nil
}
func (s *Store) SetRun(ctx context.Context, state, reason, finalError string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, "UPDATE dataset_meta SET state=?, stopping_reason=?, final_error=?, updated_at=?, completed_at=CASE WHEN ? IN ('completed','partial','failed','interrupted') THEN ? ELSE completed_at END WHERE id=1", state, reason, finalError, now, state, now)
	return err
}

func (s *Store) ReplacePlan(ctx context.Context, discovered, titles []Title, chapters []Chapter) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM titles").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("dataset plan already exists")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, t := range discovered {
		tags, err := encodeStringList(t.Tags)
		if err != nil {
			return err
		}
		languages, err := encodeStringList(t.AvailableLanguages)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO titles(id,name,url,alternative_title,original_language,status,content_rating,year,discovery_order,stratum,sample_rank,split,state,planned_chapters,provider_created_at,provider_updated_at,tags_json,available_languages_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", t.ID, t.Name, t.URL, nullString(t.AlternativeTitle), t.OriginalLanguage, t.Status, t.ContentRating, t.Year, t.DiscoveryOrder, "", -1, "", "discovered", 0, nullString(t.ProviderCreatedAt), nullString(t.ProviderUpdatedAt), nullString(tags), nullString(languages), now, now)
		if err != nil {
			return err
		}
	}
	for _, t := range titles {
		_, err = tx.ExecContext(ctx, "UPDATE titles SET stratum=?,sample_rank=?,split=?,state='planned',updated_at=? WHERE id=?", t.Stratum, t.SampleRank, t.Split, now, t.ID)
		if err != nil {
			return err
		}
	}
	for _, c := range chapters {
		_, err = tx.ExecContext(ctx, "INSERT INTO chapters(id,title_id,number,name,volume,language,release_group,published_at,url,provider_order,expected_pages,selected,output_path,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", c.ID, c.TitleID, c.Number, c.Name, c.Volume, c.Language, c.ReleaseGroup, c.PublishedAt, c.URL, c.ProviderOrder, c.ExpectedPages, 1, c.OutputPath, "planned", now, now)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "UPDATE titles SET planned_chapters=planned_chapters+1 WHERE id=?", c.TitleID)
		if err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE dataset_meta SET state='planned',updated_at=? WHERE id=1", now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PlanSummary(ctx context.Context) (Plan, bool, error) {
	var state string
	if err := s.db.QueryRowContext(ctx, "SELECT state FROM dataset_meta WHERE id=1").Scan(&state); err == sql.ErrNoRows {
		return Plan{}, false, nil
	} else if err != nil {
		return Plan{}, false, err
	}
	if state == "new" || state == "planning" {
		return Plan{}, false, nil
	}
	var plan Plan
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(CASE WHEN state!='discovered' THEN 1 ELSE 0 END),0) FROM titles").Scan(&plan.Candidates, &plan.Titles); err != nil {
		return Plan{}, false, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(SUM(expected_pages),0) FROM chapters WHERE selected=1").Scan(&plan.Chapters, &plan.EstimatedPages); err != nil {
		return Plan{}, false, err
	}
	if plan.Candidates == 0 {
		plan.Warnings = append(plan.Warnings, "no titles matched the discovery filters")
	}
	warnings, err := s.db.QueryContext(ctx, `SELECT id FROM titles WHERE state!='discovered' AND NOT EXISTS(SELECT 1 FROM chapters WHERE chapters.title_id=titles.id AND chapters.selected=1) ORDER BY id`)
	if err != nil {
		return Plan{}, false, err
	}
	for warnings.Next() {
		var titleID string
		if err := warnings.Scan(&titleID); err != nil {
			warnings.Close()
			return Plan{}, false, err
		}
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("title %q has no chapters matching the collection filters", titleID))
	}
	if err := warnings.Err(); err != nil {
		warnings.Close()
		return Plan{}, false, err
	}
	if err := warnings.Close(); err != nil {
		return Plan{}, false, err
	}
	plan.SplitCounts = map[string]int{}
	rows, err := s.db.QueryContext(ctx, "SELECT split,COUNT(*) FROM titles WHERE state!='discovered' GROUP BY split")
	if err != nil {
		return Plan{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var split string
		var count int
		if err := rows.Scan(&split, &count); err != nil {
			return Plan{}, false, err
		}
		plan.SplitCounts[split] = count
	}
	return plan, true, rows.Err()
}

func (s *Store) Planned(ctx context.Context) ([]Title, []Chapter, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,url,COALESCE(alternative_title,''),original_language,status,content_rating,year,discovery_order,stratum,sample_rank,split,state,COALESCE(provider_created_at,''),COALESCE(provider_updated_at,''),COALESCE(tags_json,''),COALESCE(available_languages_json,'') FROM titles ORDER BY sample_rank,id")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	titles := []Title{}
	for rows.Next() {
		var t Title
		var tags, languages string
		if err := rows.Scan(&t.ID, &t.Name, &t.URL, &t.AlternativeTitle, &t.OriginalLanguage, &t.Status, &t.ContentRating, &t.Year, &t.DiscoveryOrder, &t.Stratum, &t.SampleRank, &t.Split, &t.State, &t.ProviderCreatedAt, &t.ProviderUpdatedAt, &tags, &languages); err != nil {
			return nil, nil, err
		}
		if t.Tags, err = decodeStringList(tags); err != nil {
			return nil, nil, err
		}
		if t.AvailableLanguages, err = decodeStringList(languages); err != nil {
			return nil, nil, err
		}
		titles = append(titles, t)
	}
	crows, err := s.db.QueryContext(ctx, "SELECT id,title_id,number,name,volume,language,release_group,published_at,url,provider_order,expected_pages,state,COALESCE(output_path,'') FROM chapters WHERE selected=1 ORDER BY title_id,provider_order,id")
	if err != nil {
		return nil, nil, err
	}
	defer crows.Close()
	chapters := []Chapter{}
	for crows.Next() {
		var c Chapter
		if err := crows.Scan(&c.ID, &c.TitleID, &c.Number, &c.Name, &c.Volume, &c.Language, &c.ReleaseGroup, &c.PublishedAt, &c.URL, &c.ProviderOrder, &c.ExpectedPages, &c.State, &c.OutputPath); err != nil {
			return nil, nil, err
		}
		chapters = append(chapters, c)
	}
	return titles, chapters, rows.Err()
}
func (s *Store) RecordPage(ctx context.Context, page Page) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO pages(title_id,chapter_id,page_index,storage_type,relative_path,archive_entry,source_mime_type,mime_type,extension,width,height,bytes,sha256,perceptual_hash,exact_duplicate_of,near_duplicate_of,attempts,state,rejection_code,last_error,downloaded_at,validated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(chapter_id,page_index) DO UPDATE SET storage_type=excluded.storage_type,relative_path=excluded.relative_path,archive_entry=excluded.archive_entry,source_mime_type=excluded.source_mime_type,mime_type=excluded.mime_type,extension=excluded.extension,width=excluded.width,height=excluded.height,bytes=excluded.bytes,sha256=excluded.sha256,perceptual_hash=excluded.perceptual_hash,exact_duplicate_of=excluded.exact_duplicate_of,near_duplicate_of=excluded.near_duplicate_of,attempts=pages.attempts+1,state=excluded.state,rejection_code=excluded.rejection_code,last_error=excluded.last_error,validated_at=excluded.validated_at`, page.TitleID, page.ChapterID, page.Index, page.StorageType, page.RelativePath, nullString(page.ArchiveEntry), nullString(page.SourceMIMEType), page.MIMEType, nullString(page.Extension), page.Width, page.Height, page.Bytes, nullString(page.SHA256), nullString(page.PerceptualHash), nullString(page.ExactDuplicateOf), nullString(page.NearDuplicateOf), 1, page.State, nullString(page.RejectionCode), nullString(page.ErrorMessage), now, now)
	if err != nil {
		return err
	}
	var attempts int
	if err := tx.QueryRowContext(ctx, "SELECT attempts FROM pages WHERE chapter_id=? AND page_index=?", page.ChapterID, page.Index).Scan(&attempts); err != nil {
		return err
	}
	if err := recordAttempt(ctx, tx, "page", page.ChapterID+":"+strconv.Itoa(page.Index), "validate", attempts, page.State != "valid", page.ErrorMessage); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) CompleteChapter(ctx context.Context, id, output string, valid bool, message string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	state := "completed"
	if !valid {
		state = "partial"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, "UPDATE chapters SET state=?,output_path=?,last_error=?,claim_owner=NULL,claimed_at=NULL,updated_at=? WHERE id=?", state, output, nullString(message), now, id)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE titles SET completed_chapters=(SELECT COUNT(*) FROM chapters WHERE title_id=titles.id AND state='completed'),state=CASE WHEN (SELECT COUNT(*) FROM chapters WHERE title_id=titles.id AND state!='completed' AND selected=1)=0 THEN 'completed' ELSE 'partial' END,updated_at=? WHERE id=(SELECT title_id FROM chapters WHERE id=?)", now, id)
	return err
}
func (s *Store) FailChapter(ctx context.Context, id, message string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, "UPDATE chapters SET state='failed',attempts=attempts+1,last_error=?,claim_owner=NULL,claimed_at=NULL,updated_at=? WHERE id=?", message, now, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("chapter %q not found", id)
	}
	var attempts int
	if err := tx.QueryRowContext(ctx, "SELECT attempts FROM chapters WHERE id=?", id).Scan(&attempts); err != nil {
		return err
	}
	if err := recordAttempt(ctx, tx, "chapter", id, "download", attempts, false, message); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ClaimChapter(ctx context.Context, id, owner string) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if owner == "" {
		return false, fmt.Errorf("chapter claim owner cannot be empty")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE chapters SET state='downloading',claim_owner=?,claimed_at=?,attempts=attempts+1,updated_at=? WHERE id=? AND selected=1 AND state IN ('planned','partial','failed') AND claim_owner IS NULL`, owner, now, now, id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return false, err
	}
	var attempts int
	if err := tx.QueryRowContext(ctx, "SELECT attempts FROM chapters WHERE id=?", id).Scan(&attempts); err != nil {
		return false, err
	}
	if err := recordAttempt(ctx, tx, "chapter", id, "download", attempts, true, ""); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) RecordAttempt(ctx context.Context, entityType, entityID, operation string, attempt int, retryable bool, message string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return recordAttempt(ctx, s.db, entityType, entityID, operation, attempt, retryable, message)
}

func (s *Store) SetArchivePath(ctx context.Context, chapterID, archivePath string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, "UPDATE chapters SET archive_path=? WHERE id=?", archivePath, chapterID)
	return err
}

type attemptExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func recordAttempt(ctx context.Context, executor attemptExecutor, entityType, entityID, operation string, attempt int, retryable bool, message string) error {
	_, err := executor.ExecContext(ctx, "INSERT INTO attempts(entity_type,entity_id,operation,attempt,retryable,message,created_at) VALUES(?,?,?,?,?,?,?)", entityType, entityID, operation, attempt, boolToInt(retryable), nullString(message), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func (s *Store) Info(ctx context.Context) (DatasetInfo, error) {
	var data string
	var out DatasetInfo
	err := s.db.QueryRowContext(ctx, "SELECT config_json,config_hash,state,stopping_reason,created_at,updated_at FROM dataset_meta WHERE id=1").Scan(&data, &out.ConfigHash, &out.State, &out.StoppingReason, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal([]byte(data), &out.Config); err != nil {
		return out, err
	}
	q := func(query string) (int, error) {
		var n int
		err := s.db.QueryRowContext(ctx, query).Scan(&n)
		return n, err
	}
	out.Counters.DiscoveredTitles, _ = q("SELECT COUNT(*) FROM titles")
	out.Counters.PlannedTitles, _ = q("SELECT COUNT(*) FROM titles WHERE state!='discovered'")
	out.Counters.CompletedTitles, _ = q("SELECT COUNT(*) FROM titles WHERE state='completed'")
	out.Counters.FailedTitles, _ = q("SELECT COUNT(*) FROM titles WHERE state='failed'")
	out.Counters.PlannedChapters, _ = q("SELECT COUNT(*) FROM chapters WHERE selected=1")
	out.Counters.CompletedChapters, _ = q("SELECT COUNT(*) FROM chapters WHERE state='completed'")
	out.Counters.FailedChapters, _ = q("SELECT COUNT(*) FROM chapters WHERE state='failed'")
	out.Counters.PlannedPages, _ = q("SELECT COALESCE(SUM(expected_pages),0) FROM chapters WHERE selected=1")
	out.Counters.ValidPages, _ = q("SELECT COUNT(*) FROM pages WHERE state='valid'")
	out.Counters.DuplicatePages, _ = q("SELECT COUNT(*) FROM pages WHERE exact_duplicate_of IS NOT NULL")
	out.Counters.RejectedPages, _ = q("SELECT COUNT(*) FROM pages WHERE state='rejected'")
	out.Counters.FailedPages, _ = q("SELECT COUNT(*) FROM pages WHERE state='failed'")
	out.Counters.Archives, _ = q("SELECT COUNT(*) FROM chapters WHERE archive_path IS NOT NULL AND state='completed'")
	var bytes sql.NullInt64
	_ = s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(bytes),0) FROM pages WHERE state='valid'").Scan(&bytes)
	out.Counters.StoredBytes = bytes.Int64
	out.SplitCounts = map[string]int{}
	rows, err := s.db.QueryContext(ctx, "SELECT COALESCE(split,''),COUNT(*) FROM titles WHERE state!='discovered' GROUP BY split")
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var split string
		var count int
		if err := rows.Scan(&split, &count); err != nil {
			return out, err
		}
		out.SplitCounts[split] = count
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}
func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func encodeStringList(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeStringList(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil {
		return nil, fmt.Errorf("decode stored title metadata: %w", err)
	}
	return values, nil
}
