package dataset

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/evgen2571/mangate/internal/archive"
)

type ExportOptions struct {
	Split             string
	IncludeDuplicates bool
	IncludeRejected   bool
}
type ManifestRecord struct {
	SchemaVersion                                         int     `json:"schemaVersion"`
	DatasetID, Provider                                   string  `json:"datasetId"`
	Format                                                string  `json:"format"`
	StorageType                                           string  `json:"storageType"`
	TitleID, Title, TitleURL, OriginalLanguage            string  `json:"titleId,omitempty"`
	ChapterID, ChapterNumber, ChapterLanguage, ChapterURL string  `json:"chapterId,omitempty"`
	PageIndex                                             int     `json:"pageIndex"`
	RelativePath                                          string  `json:"relativePath"`
	ArchiveEntry                                          *string `json:"archiveEntry"`
	MIMEType                                              string  `json:"mimeType"`
	Width, Height                                         int     `json:"width"`
	Bytes                                                 int64   `json:"bytes"`
	SHA256, PerceptualHash, Split                         string  `json:"sha256,omitempty"`
	ExactDuplicateOf                                      *string `json:"exactDuplicateOf"`
	DownloadedAt, ValidatedAt                             string  `json:"downloadedAt,omitempty"`
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
	query := `SELECT p.title_id,t.name,t.url,t.original_language,p.chapter_id,c.number,c.language,c.url,p.page_index,p.storage_type,p.relative_path,p.archive_entry,p.mime_type,p.width,p.height,p.bytes,p.sha256,p.perceptual_hash,p.exact_duplicate_of,t.split,p.downloaded_at,p.validated_at FROM pages p JOIN titles t ON t.id=p.title_id JOIN chapters c ON c.id=p.chapter_id WHERE p.state='valid'`
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
		var archiveEntry, duplicate sqlString
		var downloaded, validated sqlString
		if err := rows.Scan(&r.TitleID, &r.Title, &r.TitleURL, &r.OriginalLanguage, &r.ChapterID, &r.ChapterNumber, &r.ChapterLanguage, &r.ChapterURL, &r.PageIndex, &r.StorageType, &r.RelativePath, &archiveEntry, &r.MIMEType, &r.Width, &r.Height, &r.Bytes, &r.SHA256, &r.PerceptualHash, &duplicate, &r.Split, &downloaded, &validated); err != nil {
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
	summary := map[string]any{"schemaVersion": 1, "datasetId": info.Config.DatasetID, "provider": info.Config.Provider, "format": info.Config.Output.Format, "configurationHash": info.ConfigHash, "state": info.State, "stoppingReason": info.StoppingReason, "createdAt": info.CreatedAt, "updatedAt": info.UpdatedAt, "counters": info.Counters, "databaseSchemaVersion": SchemaVersion, "manifestSchemaVersion": 1}
	return writeJSONFile(filepath.Join(store.Root(), "summary.json"), summary)
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
	for rows.Next() {
		var titleID, chapterID, path, hash string
		var index int
		var entry sqlString
		if err := rows.Scan(&titleID, &chapterID, &index, &path, &entry, &hash); err != nil {
			return nil, err
		}
		checked++
		if entry.Valid {
			if err := verifyArchiveEntry(filepath.Join(store.Root(), filepath.FromSlash(path)), entry.String, hash, info.Config.Validation); err != nil {
				invalid++
				if repair {
					_, _ = store.db.ExecContext(ctx, "UPDATE pages SET state='pending',last_error='archive missing' WHERE chapter_id=? AND page_index=?", chapterID, index)
				}
			}
			continue
		}
		validated, _, err := ValidateFile(filepath.Join(store.Root(), filepath.FromSlash(path)), info.Config.Validation)
		if err != nil || (hash != "" && validated.SHA256 != hash) {
			invalid++
			if repair {
				_, _ = store.db.ExecContext(ctx, "UPDATE pages SET state='pending',last_error='file missing or modified' WHERE chapter_id=? AND page_index=?", chapterID, index)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if repair {
		if _, err := store.db.ExecContext(ctx, `UPDATE chapters SET state='partial' WHERE id IN (SELECT DISTINCT chapter_id FROM pages WHERE state='pending')`); err != nil {
			return nil, err
		}
		if _, err := store.db.ExecContext(ctx, `UPDATE titles SET state=CASE WHEN EXISTS(SELECT 1 FROM chapters WHERE chapters.title_id=titles.id AND selected=1 AND state!='completed') THEN 'partial' ELSE 'completed' END`); err != nil {
			return nil, err
		}
		if err := Export(ctx, store, ExportOptions{}); err != nil {
			return nil, err
		}
	}
	return map[string]any{"datasetRoot": store.Root(), "checkedPages": checked, "invalidPages": invalid, "repair": repair, "valid": invalid == 0}, nil
}

func verifyArchiveEntry(path, entry, expectedHash string, validation Validation) error {
	inspection, err := archive.Inspect(path)
	if err != nil || !inspection.Complete {
		if err == nil {
			err = fmt.Errorf("archive is incomplete")
		}
		return err
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
		source, err := file.Open()
		if err != nil {
			return err
		}
		temp, err := os.CreateTemp("", "mangate-archive-entry-*")
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(temp, source)
		closeErr := temp.Close()
		_ = source.Close()
		if copyErr != nil {
			_ = os.Remove(temp.Name())
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(temp.Name())
			return closeErr
		}
		validated, _, err := ValidateFile(temp.Name(), validation)
		_ = os.Remove(temp.Name())
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
	rows, err := store.db.QueryContext(ctx, "SELECT chapter_id,page_index,state,COALESCE(rejection_code,''),COALESCE(last_error,'') FROM pages WHERE state IN ('rejected','failed') ORDER BY chapter_id,page_index")
	if err != nil {
		return err
	}
	defer rows.Close()
	path := filepath.Join(store.Root(), "reports", "failures.jsonl")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".failures-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	w := bufio.NewWriter(tmp)
	for rows.Next() {
		var chapterID, state, code, message string
		var index int
		if err := rows.Scan(&chapterID, &index, &state, &code, &message); err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]any{"chapterId": chapterID, "pageIndex": index, "state": state, "code": code, "message": strings.TrimSpace(message)})
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
