package dataset

import (
	"archive/zip"
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/evgen2571/mangate/internal/archive"
)

type ExportOptions struct {
	Split             string `json:"split"`
	IncludeDuplicates bool   `json:"includeDuplicates"`
	IncludeRejected   bool   `json:"includeRejected"`
}
type ManifestRecord struct {
	SchemaVersion    int     `json:"schemaVersion"`
	DatasetID        string  `json:"datasetId"`
	Provider         string  `json:"provider"`
	Format           string  `json:"format"`
	StorageType      string  `json:"storageType"`
	TitleID          string  `json:"titleId,omitempty"`
	Title            string  `json:"title,omitempty"`
	TitleURL         string  `json:"titleUrl,omitempty"`
	OriginalLanguage string  `json:"originalLanguage,omitempty"`
	ChapterID        string  `json:"chapterId,omitempty"`
	ChapterNumber    string  `json:"chapterNumber,omitempty"`
	ChapterLanguage  string  `json:"chapterLanguage,omitempty"`
	ChapterURL       string  `json:"chapterUrl,omitempty"`
	PageIndex        int     `json:"pageIndex"`
	RelativePath     string  `json:"relativePath"`
	ArchiveEntry     *string `json:"archiveEntry"`
	SourceMIMEType   string  `json:"sourceMimeType,omitempty"`
	MIMEType         string  `json:"mimeType"`
	Extension        string  `json:"extension,omitempty"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	Bytes            int64   `json:"bytes"`
	SHA256           string  `json:"sha256,omitempty"`
	PerceptualHash   string  `json:"perceptualHash,omitempty"`
	Split            string  `json:"split,omitempty"`
	ExactDuplicateOf *string `json:"exactDuplicateOf"`
	NearDuplicateOf  *string `json:"nearDuplicateOf"`
	DownloadedAt     string  `json:"downloadedAt,omitempty"`
	ValidatedAt      string  `json:"validatedAt,omitempty"`
}

func Export(ctx context.Context, store *Store, options ExportOptions) error {
	info, err := store.Info(ctx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(store.Root(), "reports"), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(store.Root(), ".manifest-*.jsonl")
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp.Name())
		}
	}()
	writer := bufio.NewWriter(tmp)
	query := `SELECT p.title_id,t.name,COALESCE(t.url,''),COALESCE(t.original_language,''),p.chapter_id,COALESCE(c.number,''),COALESCE(c.language,''),COALESCE(c.url,''),p.page_index,COALESCE(p.storage_type,''),COALESCE(p.relative_path,''),p.archive_entry,COALESCE(p.source_mime_type,''),COALESCE(p.mime_type,''),COALESCE(p.extension,''),COALESCE(p.width,0),COALESCE(p.height,0),COALESCE(p.bytes,0),COALESCE(p.sha256,''),COALESCE(p.perceptual_hash,''),p.exact_duplicate_of,p.near_duplicate_of,COALESCE(t.split,''),COALESCE(p.downloaded_at,''),COALESCE(p.validated_at,'') FROM pages p JOIN titles t ON t.id=p.title_id JOIN chapters c ON c.id=p.chapter_id WHERE p.state='valid'`
	if options.IncludeRejected {
		query = strings.Replace(query, "p.state='valid'", "p.state IN ('valid','rejected')", 1)
	}
	args := []any{}
	if options.Split != "" {
		query += " AND t.split=?"
		args = append(args, options.Split)
	}
	if !options.IncludeDuplicates {
		query += " AND p.exact_duplicate_of IS NULL"
	}
	query += " ORDER BY p.title_id,p.chapter_id,p.page_index"
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r ManifestRecord
		var archiveEntry, duplicate, nearDuplicate sqlString
		var downloaded, validated sqlString
		if err := rows.Scan(&r.TitleID, &r.Title, &r.TitleURL, &r.OriginalLanguage, &r.ChapterID, &r.ChapterNumber, &r.ChapterLanguage, &r.ChapterURL, &r.PageIndex, &r.StorageType, &r.RelativePath, &archiveEntry, &r.SourceMIMEType, &r.MIMEType, &r.Extension, &r.Width, &r.Height, &r.Bytes, &r.SHA256, &r.PerceptualHash, &duplicate, &nearDuplicate, &r.Split, &downloaded, &validated); err != nil {
			return err
		}
		r.SchemaVersion = 1
		r.DatasetID = info.Config.DatasetID
		r.Provider = info.Config.Provider
		r.Format = string(info.Config.Output.Format)
		if archiveEntry.Valid {
			value := archiveEntry.String
			r.ArchiveEntry = &value
		}
		if duplicate.Valid {
			value := duplicate.String
			r.ExactDuplicateOf = &value
		}
		if nearDuplicate.Valid {
			value := nearDuplicate.String
			r.NearDuplicateOf = &value
		}
		r.DownloadedAt, r.ValidatedAt = downloaded.String, validated.String
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(store.Root(), "manifest.jsonl")); err != nil {
		return err
	}
	ok = true
	stats, err := summaryStatistics(ctx, store)
	if err != nil {
		return err
	}
	summary := map[string]any{"schemaVersion": 1, "datasetId": info.Config.DatasetID, "provider": info.Config.Provider, "format": info.Config.Output.Format, "configurationHash": info.ConfigHash, "state": info.State, "stoppingReason": info.StoppingReason, "createdAt": info.CreatedAt, "updatedAt": info.UpdatedAt, "counters": info.Counters, "statistics": stats, "databaseSchemaVersion": SchemaVersion, "manifestSchemaVersion": 1, "manifestOptions": map[string]any{"split": options.Split, "includeDuplicates": options.IncludeDuplicates, "includeRejected": options.IncludeRejected}}
	return writeJSONFile(filepath.Join(store.Root(), "summary.json"), summary)
}

func summaryStatistics(ctx context.Context, store *Store) (map[string]any, error) {
	stats := map[string]any{"splitCounts": map[string]int{}, "rejectionCategories": map[string]int{}, "failureCategories": map[string]int{}}
	var averageWidth, averageHeight, averageAspect float64
	var minWidth, maxWidth, minHeight, maxHeight int
	if err := store.db.QueryRowContext(ctx, `SELECT COALESCE(AVG(width),0),COALESCE(MIN(width),0),COALESCE(MAX(width),0),COALESCE(AVG(height),0),COALESCE(MIN(height),0),COALESCE(MAX(height),0),COALESCE(AVG(CAST(width AS REAL)/NULLIF(height,0)),0) FROM pages WHERE state='valid'`).Scan(&averageWidth, &minWidth, &maxWidth, &averageHeight, &minHeight, &maxHeight, &averageAspect); err != nil {
		return nil, err
	}
	stats["width"] = map[string]any{"average": averageWidth, "minimum": minWidth, "maximum": maxWidth}
	stats["height"] = map[string]any{"average": averageHeight, "minimum": minHeight, "maximum": maxHeight}
	stats["aspectRatio"] = map[string]any{"average": averageAspect}
	var validPages, titleCount, chapterCount int
	if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*),COUNT(DISTINCT title_id),COUNT(DISTINCT chapter_id) FROM pages WHERE state='valid'").Scan(&validPages, &titleCount, &chapterCount); err != nil {
		return nil, err
	}
	stats["pagesPerTitle"] = ratio(validPages, titleCount)
	stats["pagesPerChapter"] = ratio(validPages, chapterCount)
	if err := collectCountMap(ctx, store, "SELECT COALESCE(split,''),COUNT(*) FROM titles WHERE state!='discovered' GROUP BY split", stats["splitCounts"].(map[string]int)); err != nil {
		return nil, err
	}
	if err := collectCountMap(ctx, store, "SELECT COALESCE(rejection_code,''),COUNT(*) FROM pages WHERE state='rejected' GROUP BY rejection_code", stats["rejectionCategories"].(map[string]int)); err != nil {
		return nil, err
	}
	if err := collectCountMap(ctx, store, "SELECT COALESCE(last_error,''),COUNT(*) FROM chapters WHERE state='failed' GROUP BY last_error", stats["failureCategories"].(map[string]int)); err != nil {
		return nil, err
	}
	var retries int
	if err := store.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(CASE WHEN attempt>1 THEN attempt-1 ELSE 0 END),0) FROM attempts").Scan(&retries); err != nil {
		return nil, err
	}
	stats["retryCount"] = retries
	var archiveBytes int64
	rows, err := store.db.QueryContext(ctx, "SELECT archive_path FROM chapters WHERE archive_path IS NOT NULL AND state='completed'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		if info, err := os.Stat(path); err == nil {
			archiveBytes += info.Size()
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	stats["archiveBytes"] = archiveBytes
	return stats, nil
}
func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
func collectCountMap(ctx context.Context, store *Store, query string, target map[string]int) error {
	rows, err := store.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return err
		}
		target[name] = count
	}
	return rows.Err()
}

type sqlString struct {
	String string
	Valid  bool
}

func (s *sqlString) Scan(value any) error {
	if value == nil {
		s.String = ""
		s.Valid = false
		return nil
	}
	switch value := value.(type) {
	case string:
		s.String = value
	case []byte:
		s.String = string(value)
	default:
		return fmt.Errorf("scan text %T", value)
	}
	s.Valid = true
	return nil
}
func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".part"
	if err := os.WriteFile(temp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func Verify(ctx context.Context, store *Store, repair bool) (map[string]any, error) {
	info, err := store.Info(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := store.db.QueryContext(ctx, "SELECT title_id,chapter_id,page_index,relative_path,archive_entry,sha256 FROM pages WHERE state='valid' ORDER BY title_id,chapter_id,page_index")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	checked, invalid := 0, 0
	invalidArchives := map[string]struct{}{}
	for rows.Next() {
		var titleID, chapterID, path, hash string
		var index int
		var entry sqlString
		if err := rows.Scan(&titleID, &chapterID, &index, &path, &entry, &hash); err != nil {
			return nil, err
		}
		checked++
		if entry.Valid {
			if _, alreadyInvalid := invalidArchives[chapterID]; alreadyInvalid {
				continue
			}
			archivePath := filepath.Join(store.Root(), filepath.FromSlash(path))
			archiveErr := verifyArchiveEntry(archivePath, titleID, chapterID, entry.String, hash, info.Config.Validation)
			if !info.Config.Output.Format.IsArchive() || !strings.EqualFold(filepath.Ext(archivePath), info.Config.Output.Format.Extension()) || archiveErr != nil {
				invalid++
				if repair {
					invalidArchives[chapterID] = struct{}{}
					if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
						return nil, fmt.Errorf("remove corrupt archive %q: %w", archivePath, err)
					}
					if _, err := store.db.ExecContext(ctx, "UPDATE pages SET state='pending',last_error='archive missing or modified' WHERE chapter_id=?", chapterID); err != nil {
						return nil, err
					}
					if _, err := store.db.ExecContext(ctx, "UPDATE chapters SET state='partial',archive_path=NULL,last_error='archive missing or modified',claim_owner=NULL,claimed_at=NULL WHERE id=?", chapterID); err != nil {
						return nil, err
					}
				}
			}
			continue
		}
		if info.Config.Output.Format.IsArchive() {
			invalid++
			if repair {
				_, _ = store.db.ExecContext(ctx, "UPDATE pages SET state='pending',last_error='archive page is stored as a file' WHERE chapter_id=? AND page_index=?", chapterID, index)
			}
			continue
		}
		validated, _, err := ValidateFile(filepath.Join(store.Root(), filepath.FromSlash(path)), info.Config.Validation)
		if err != nil || (hash != "" && validated.SHA256 != hash) || !matchesOutputFormat(path, validated.MIMEType, info.Config.Output.Format) {
			invalid++
			if repair {
				_, _ = store.db.ExecContext(ctx, "UPDATE pages SET state='pending',last_error='file missing or modified' WHERE chapter_id=? AND page_index=?", chapterID, index)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	adoptedArchives := 0
	if repair {
		adoptedArchives, err = adoptCompletedArchives(ctx, store, info.Config)
		if err != nil {
			return nil, err
		}
		temporary, err := temporaryFiles(store.Root())
		if err != nil {
			return nil, err
		}
		for _, path := range temporary {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("remove abandoned temporary file %q: %w", path, err)
			}
		}
		staging, err := archiveStagingDirectories(store.Root())
		if err != nil {
			return nil, err
		}
		for _, path := range staging {
			if err := os.RemoveAll(path); err != nil {
				return nil, fmt.Errorf("remove abandoned archive staging directory %q: %w", path, err)
			}
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE chapters SET state='partial', claim_owner=NULL, claimed_at=NULL, last_error=COALESCE(last_error, 'interrupted work claim') WHERE state='downloading' AND claim_owner IS NOT NULL`); err != nil {
			return nil, err
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE chapters SET state='partial' WHERE id IN (SELECT DISTINCT chapter_id FROM pages WHERE state='pending')`); err != nil {
			return nil, err
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE chapters SET state='partial', last_error=COALESCE(last_error, 'completed chapter has incomplete page state') WHERE selected=1 AND state='completed' AND expected_pages>0 AND expected_pages!=(SELECT COUNT(*) FROM pages WHERE pages.chapter_id=chapters.id AND pages.state='valid')`); err != nil {
			return nil, err
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE titles SET state=CASE WHEN EXISTS(SELECT 1 FROM chapters WHERE chapters.title_id=titles.id AND selected=1 AND state!='completed') THEN 'partial' ELSE 'completed' END`); err != nil {
			return nil, err
		}
		if err := Export(ctx, store, ExportOptions{}); err != nil {
			return nil, err
		}
	}
	var splitLeakage int
	err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT p.sha256 FROM pages p JOIN titles t ON t.id=p.title_id WHERE p.state='valid' AND p.sha256 IS NOT NULL AND t.split<>'' GROUP BY p.sha256 HAVING COUNT(DISTINCT t.split)>1)`).Scan(&splitLeakage)
	if err != nil {
		return nil, err
	}
	stateInconsistencies, err := stateInconsistencies(ctx, store)
	if err != nil {
		return nil, err
	}
	duplicateReferences, err := invalidDuplicateReferences(ctx, store)
	if err != nil {
		return nil, err
	}
	manifestInconsistencies, err := verifyManifest(ctx, store, info)
	if err != nil {
		return nil, err
	}
	temporary, err := temporaryFiles(store.Root())
	if err != nil {
		return nil, err
	}
	staging, err := archiveStagingDirectories(store.Root())
	if err != nil {
		return nil, err
	}
	unexpected, err := unexpectedDataFiles(ctx, store)
	if err != nil {
		return nil, err
	}
	return map[string]any{"datasetRoot": store.Root(), "checkedPages": checked, "invalidPages": invalid, "adoptedArchives": adoptedArchives, "stateInconsistencies": stateInconsistencies, "invalidDuplicateReferences": duplicateReferences, "manifestInconsistencies": manifestInconsistencies, "temporaryFiles": len(temporary), "stagingDirectories": len(staging), "unexpectedFiles": len(unexpected), "splitLeakage": splitLeakage, "repair": repair, "valid": invalid == 0 && stateInconsistencies == 0 && duplicateReferences == 0 && manifestInconsistencies == 0 && splitLeakage == 0 && len(temporary) == 0 && len(staging) == 0 && len(unexpected) == 0}, nil
}

func matchesOutputFormat(path, mimeType string, format archive.Format) bool {
	switch format {
	case archive.FormatPNG:
		return strings.EqualFold(filepath.Ext(path), ".png") && mimeType == "image/png"
	case archive.FormatJPEG:
		return strings.EqualFold(filepath.Ext(path), ".jpeg") && mimeType == "image/jpeg"
	case archive.FormatDirectory:
		return true
	default:
		return false
	}
}

func stateInconsistencies(ctx context.Context, store *Store) (int, error) {
	var incompleteChapters, incompleteTitles int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chapters WHERE selected=1 AND state='completed' AND expected_pages>0 AND expected_pages!=(SELECT COUNT(*) FROM pages WHERE pages.chapter_id=chapters.id AND pages.state='valid')`).Scan(&incompleteChapters); err != nil {
		return 0, err
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM titles WHERE state='completed' AND EXISTS(SELECT 1 FROM chapters WHERE chapters.title_id=titles.id AND chapters.selected=1 AND chapters.state!='completed')`).Scan(&incompleteTitles); err != nil {
		return 0, err
	}
	return incompleteChapters + incompleteTitles, nil
}

func invalidDuplicateReferences(ctx context.Context, store *Store) (int, error) {
	var invalid int
	query := `SELECT COUNT(*) FROM pages p WHERE p.state='valid' AND (
		(p.exact_duplicate_of IS NOT NULL AND (instr(p.exact_duplicate_of,':')=0 OR NOT EXISTS(SELECT 1 FROM pages original WHERE original.state='valid' AND original.chapter_id=substr(p.exact_duplicate_of,1,instr(p.exact_duplicate_of,':')-1) AND original.page_index=CAST(substr(p.exact_duplicate_of,instr(p.exact_duplicate_of,':')+1) AS INTEGER)))) OR
		(p.near_duplicate_of IS NOT NULL AND (instr(p.near_duplicate_of,':')=0 OR NOT EXISTS(SELECT 1 FROM pages original WHERE original.state='valid' AND original.chapter_id=substr(p.near_duplicate_of,1,instr(p.near_duplicate_of,':')-1) AND original.page_index=CAST(substr(p.near_duplicate_of,instr(p.near_duplicate_of,':')+1) AS INTEGER))))
	)`
	if err := store.db.QueryRowContext(ctx, query).Scan(&invalid); err != nil {
		return 0, err
	}
	return invalid, nil
}

func verifyManifest(ctx context.Context, store *Store, info DatasetInfo) (int, error) {
	options, err := manifestExportOptions(store.Root())
	if err != nil {
		return 0, err
	}
	path := filepath.Join(store.Root(), "manifest.jsonl")
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()
	inconsistencies := 0
	lines := 0
	var previousTitle, previousChapter string
	previousPage := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		lines++
		var record ManifestRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			inconsistencies++
			continue
		}
		if record.SchemaVersion != 1 || record.DatasetID != info.Config.DatasetID || record.Provider != info.Config.Provider || record.Format != string(info.Config.Output.Format) || !manifestOrderAfter(previousTitle, previousChapter, previousPage, record.TitleID, record.ChapterID, record.PageIndex) {
			inconsistencies++
			continue
		}
		if ok, err := manifestRecordMatches(ctx, store, options, record); err != nil {
			return 0, err
		} else if !ok {
			inconsistencies++
			continue
		}
		previousTitle, previousChapter, previousPage = record.TitleID, record.ChapterID, record.PageIndex
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	expected, err := manifestRecordCount(ctx, store, options)
	if err != nil {
		return 0, err
	}
	if lines != expected {
		inconsistencies += abs(lines - expected)
	}
	return inconsistencies, nil
}

func manifestExportOptions(root string) (ExportOptions, error) {
	data, err := os.ReadFile(filepath.Join(root, "summary.json"))
	if os.IsNotExist(err) {
		return ExportOptions{}, nil
	}
	if err != nil {
		return ExportOptions{}, err
	}
	var summary struct {
		ManifestOptions ExportOptions `json:"manifestOptions"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		return ExportOptions{}, fmt.Errorf("decode dataset summary: %w", err)
	}
	return summary.ManifestOptions, nil
}

func manifestOrderAfter(previousTitle, previousChapter string, previousPage int, title, chapter string, page int) bool {
	if previousTitle == "" && previousChapter == "" && previousPage == 0 {
		return true
	}
	if title != previousTitle {
		return title > previousTitle
	}
	if chapter != previousChapter {
		return chapter > previousChapter
	}
	return page > previousPage
}

func manifestRecordMatches(ctx context.Context, store *Store, options ExportOptions, record ManifestRecord) (bool, error) {
	query := `SELECT COALESCE(p.storage_type,''),COALESCE(p.relative_path,''),COALESCE(p.archive_entry,''),COALESCE(p.source_mime_type,''),COALESCE(p.mime_type,''),COALESCE(p.extension,''),COALESCE(p.width,0),COALESCE(p.height,0),COALESCE(p.bytes,0),COALESCE(p.sha256,''),COALESCE(p.perceptual_hash,''),COALESCE(p.exact_duplicate_of,''),COALESCE(p.near_duplicate_of,''),COALESCE(t.split,'') FROM pages p JOIN titles t ON t.id=p.title_id WHERE p.title_id=? AND p.chapter_id=? AND p.page_index=? AND p.state='valid'`
	if options.IncludeRejected {
		query = strings.Replace(query, "p.state='valid'", "p.state IN ('valid','rejected')", 1)
	}
	args := []any{record.TitleID, record.ChapterID, record.PageIndex}
	if options.Split != "" {
		query += " AND t.split=?"
		args = append(args, options.Split)
	}
	if !options.IncludeDuplicates {
		query += " AND p.exact_duplicate_of IS NULL"
	}
	var storage, path, entry, sourceMIME, mimeType, extension, sha, perceptual, duplicate, nearDuplicate, split string
	var width, height int
	var bytes int64
	err := store.db.QueryRowContext(ctx, query, args...).Scan(&storage, &path, &entry, &sourceMIME, &mimeType, &extension, &width, &height, &bytes, &sha, &perceptual, &duplicate, &nearDuplicate, &split)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	archiveEntry := ""
	if record.ArchiveEntry != nil {
		archiveEntry = *record.ArchiveEntry
	}
	exactDuplicate := ""
	if record.ExactDuplicateOf != nil {
		exactDuplicate = *record.ExactDuplicateOf
	}
	near := ""
	if record.NearDuplicateOf != nil {
		near = *record.NearDuplicateOf
	}
	return storage == record.StorageType && path == record.RelativePath && entry == archiveEntry && sourceMIME == record.SourceMIMEType && mimeType == record.MIMEType && extension == record.Extension && width == record.Width && height == record.Height && bytes == record.Bytes && sha == record.SHA256 && perceptual == record.PerceptualHash && duplicate == exactDuplicate && nearDuplicate == near && split == record.Split, nil
}

func manifestRecordCount(ctx context.Context, store *Store, options ExportOptions) (int, error) {
	query := "SELECT COUNT(*) FROM pages p JOIN titles t ON t.id=p.title_id WHERE p.state='valid'"
	if options.IncludeRejected {
		query = strings.Replace(query, "p.state='valid'", "p.state IN ('valid','rejected')", 1)
	}
	args := []any{}
	if options.Split != "" {
		query += " AND t.split=?"
		args = append(args, options.Split)
	}
	if !options.IncludeDuplicates {
		query += " AND p.exact_duplicate_of IS NULL"
	}
	var count int
	if err := store.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func temporaryFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".part") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// archiveStagingDirectories returns the leaf staging directories that contain
// interrupted archive page output. The .staging hierarchy itself is not an
// error when it is empty.
func archiveStagingDirectories(root string) ([]string, error) {
	stagingRoot := filepath.Join(root, ".staging")
	if _, err := os.Stat(stagingRoot); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	directories := map[string]struct{}{}
	err := filepath.WalkDir(stagingRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		directories[filepath.Dir(path)] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	staging := make([]string, 0, len(directories))
	for path := range directories {
		staging = append(staging, path)
	}
	slices.Sort(staging)
	return staging, nil
}

// adoptCompletedArchives reconciles the narrow interruption window after an
// archive is finalized but before its page records are committed. It accepts
// only the canonical archive for a planned, incomplete chapter and validates
// every page before making the database state complete.
func adoptCompletedArchives(ctx context.Context, store *Store, cfg Config) (int, error) {
	if !cfg.Output.Format.IsArchive() {
		return 0, nil
	}
	rows, err := store.db.QueryContext(ctx, `SELECT c.id,c.title_id,c.expected_pages,COALESCE(t.split,'') FROM chapters c JOIN titles t ON t.id=c.title_id WHERE c.selected=1 AND c.state!='completed' ORDER BY c.title_id,c.provider_order,c.id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	adopted := 0
	for rows.Next() {
		var chapterID, titleID, split string
		var expectedPages int
		if err := rows.Scan(&chapterID, &titleID, &expectedPages, &split); err != nil {
			return 0, err
		}
		if expectedPages <= 0 {
			continue
		}
		archivePath := filepath.Join(store.Root(), "data", safeSegment(cfg.Provider), safeSegment(titleID), safeSegment(chapterID)+cfg.Output.Format.Extension())
		inspection, err := archive.Inspect(archivePath)
		if err != nil || !inspection.Valid || !inspection.Complete || !inspection.IdentityConfirmed || inspection.TitleID != titleID || inspection.ChapterID != chapterID || inspection.PageCount != expectedPages || len(inspection.UnexpectedEntries) > 0 {
			continue
		}
		if inspection.Metadata == nil || inspection.Metadata.Provider != cfg.Provider {
			continue
		}
		pages, err := validateArchivePages(archivePath, cfg.Validation)
		if err != nil {
			continue
		}
		relative, err := filepath.Rel(store.Root(), archivePath)
		if err != nil {
			return 0, err
		}
		for index, page := range pages {
			if err := store.RecordPage(ctx, Page{TitleID: titleID, ChapterID: chapterID, Index: index + 1, StorageType: "archive", RelativePath: filepath.ToSlash(relative), ArchiveEntry: page.entry, SourceMIMEType: page.image.MIMEType, MIMEType: page.image.MIMEType, Extension: filepath.Ext(page.entry), Width: page.image.Width, Height: page.image.Height, Bytes: page.image.Bytes, SHA256: page.image.SHA256, PerceptualHash: page.image.PerceptualHash, State: "valid", Split: split}); err != nil {
				return 0, err
			}
		}
		if err := store.CompleteChapter(ctx, chapterID, archivePath, true, ""); err != nil {
			return 0, err
		}
		if _, err := store.db.ExecContext(ctx, "UPDATE chapters SET archive_path=? WHERE id=?", archivePath, chapterID); err != nil {
			return 0, err
		}
		adopted++
	}
	return adopted, rows.Err()
}

type archivePage struct {
	entry string
	image ValidatedImage
}

func validateArchivePages(path string, validation Validation) ([]archivePage, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	pages := []archivePage{}
	for _, file := range reader.File {
		if file.Name == ".mangate.json" || file.Name == "ComicInfo.xml" {
			continue
		}
		if filepath.Dir(file.Name) != "." || !isArchiveImageName(file.Name) {
			return nil, fmt.Errorf("unexpected archive entry %q", file.Name)
		}
		image, err := validateArchiveFile(file, validation)
		if err != nil {
			return nil, err
		}
		pages = append(pages, archivePage{entry: file.Name, image: image})
	}
	return pages, nil
}

func isArchiveImageName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".jfif", ".png", ".gif", ".webp", ".avif", ".bmp", ".img":
		return true
	default:
		return false
	}
}

func validateArchiveFile(file *zip.File, validation Validation) (ValidatedImage, error) {
	source, err := file.Open()
	if err != nil {
		return ValidatedImage{}, err
	}
	defer source.Close()
	temp, err := os.CreateTemp("", "mangate-archive-entry-*")
	if err != nil {
		return ValidatedImage{}, err
	}
	defer os.Remove(temp.Name())
	if _, err := io.Copy(temp, source); err != nil {
		_ = temp.Close()
		return ValidatedImage{}, err
	}
	if err := temp.Close(); err != nil {
		return ValidatedImage{}, err
	}
	validated, _, err := ValidateFile(temp.Name(), validation)
	return validated, err
}

func unexpectedDataFiles(ctx context.Context, store *Store) ([]string, error) {
	expected := map[string]struct{}{}
	rows, err := store.db.QueryContext(ctx, "SELECT DISTINCT relative_path FROM pages WHERE state='valid' AND relative_path IS NOT NULL AND relative_path!=''")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			rows.Close()
			return nil, err
		}
		expected[filepath.ToSlash(path)] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	dataRoot := filepath.Join(store.Root(), "data")
	if _, err := os.Stat(dataRoot); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	unexpected := []string{}
	err = filepath.WalkDir(dataRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(store.Root(), path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if _, ok := expected[relative]; ok {
			return nil
		}
		name := entry.Name()
		if name == "title.json" || name == "chapter.json" || name == ".mangate.json" || strings.HasSuffix(name, ".cbz.json") || strings.HasSuffix(name, ".zip.json") {
			return nil
		}
		unexpected = append(unexpected, relative)
		return nil
	})
	return unexpected, err
}

func verifyArchiveEntry(path, expectedTitleID, expectedChapterID, entry, expectedHash string, validation Validation) error {
	inspection, err := archive.Inspect(path)
	if err != nil || !inspection.Complete {
		if err == nil {
			err = fmt.Errorf("archive is incomplete")
		}
		return err
	}
	if !inspection.IdentityConfirmed || inspection.TitleID != expectedTitleID || inspection.ChapterID != expectedChapterID {
		return fmt.Errorf("archive identity does not match title %q chapter %q", expectedTitleID, expectedChapterID)
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != entry {
			continue
		}
		validated, err := validateArchiveFile(file, validation)
		if err != nil {
			return err
		}
		if expectedHash != "" && validated.SHA256 != expectedHash {
			return fmt.Errorf("archive entry checksum differs")
		}
		return nil
	}
	return fmt.Errorf("archive entry %q is missing", entry)
}

func Failures(ctx context.Context, store *Store) error {
	rows, err := store.db.QueryContext(ctx, `SELECT entity_type,chapter_id,page_index,state,code,message FROM (
		SELECT 'chapter' AS entity_type,id AS chapter_id,0 AS page_index,state,'' AS code,COALESCE(last_error,'') AS message FROM chapters WHERE state='failed'
		UNION ALL
		SELECT 'page' AS entity_type,chapter_id,page_index,state,COALESCE(rejection_code,''),COALESCE(last_error,'') FROM pages WHERE state IN ('rejected','failed')
	) ORDER BY chapter_id,entity_type,page_index`)
	if err != nil {
		return err
	}
	defer rows.Close()
	path := filepath.Join(store.Root(), "reports", "failures.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".failures-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	w := bufio.NewWriter(tmp)
	for rows.Next() {
		var entityType, chapterID, state, code, message string
		var index int
		if err := rows.Scan(&entityType, &chapterID, &index, &state, &code, &message); err != nil {
			return err
		}
		record := map[string]any{"entityType": entityType, "chapterId": chapterID, "state": state, "code": code, "message": strings.TrimSpace(message)}
		if entityType == "page" {
			record["pageIndex"] = index
		}
		data, _ := json.Marshal(record)
		_, _ = w.Write(append(data, '\n'))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
