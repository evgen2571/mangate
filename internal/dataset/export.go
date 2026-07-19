package dataset

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func manifestRecords(store *Store, options ExportOptions) ([]ManifestRecord, error) {
	info, err := store.Info(context.Background())
	if err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	records := []ManifestRecord{}
	for _, page := range store.data.Pages {
		if page.State != "valid" && !(options.IncludeRejected && page.State == "rejected") {
			continue
		}
		if !options.IncludeDuplicates && page.ExactDuplicateOf != "" {
			continue
		}
		title, titleOK := store.data.Titles[page.TitleID]
		chapter, chapterOK := store.data.Chapters[page.ChapterID]
		if !titleOK || !chapterOK || (options.Split != "" && title.Split != options.Split) {
			continue
		}
		records = append(records, ManifestRecord{SchemaVersion: 1, DatasetID: info.Config.DatasetID, Provider: info.Config.Provider, Format: string(info.Config.Output.Format), StorageType: page.StorageType, TitleID: page.TitleID, Title: title.Name, TitleURL: title.URL, OriginalLanguage: title.OriginalLanguage, ChapterID: page.ChapterID, ChapterNumber: chapter.Number, ChapterLanguage: chapter.Language, ChapterURL: chapter.URL, PageIndex: page.Index, RelativePath: page.RelativePath, ArchiveEntry: optional(page.ArchiveEntry), SourceMIMEType: page.SourceMIMEType, MIMEType: page.MIMEType, Extension: page.Extension, Width: page.Width, Height: page.Height, Bytes: page.Bytes, SHA256: page.SHA256, PerceptualHash: page.PerceptualHash, Split: title.Split, ExactDuplicateOf: optional(page.ExactDuplicateOf), NearDuplicateOf: optional(page.NearDuplicateOf), DownloadedAt: page.DownloadedAt, ValidatedAt: page.ValidatedAt})
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].TitleID != records[j].TitleID {
			return records[i].TitleID < records[j].TitleID
		}
		if records[i].ChapterID != records[j].ChapterID {
			return records[i].ChapterID < records[j].ChapterID
		}
		return records[i].PageIndex < records[j].PageIndex
	})
	return records, nil
}
func Export(_ context.Context, store *Store, options ExportOptions) error {
	records, err := manifestRecords(store, options)
	if err != nil {
		return err
	}
	info, err := store.Info(context.Background())
	if err != nil {
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
	w := bufio.NewWriter(tmp)
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(store.Root(), "manifest.jsonl")); err != nil {
		return err
	}
	ok = true
	stats := statistics(store)
	summary := map[string]any{"schemaVersion": 1, "datasetId": info.Config.DatasetID, "provider": info.Config.Provider, "format": info.Config.Output.Format, "configurationHash": info.ConfigHash, "state": info.State, "stoppingReason": info.StoppingReason, "createdAt": info.CreatedAt, "updatedAt": info.UpdatedAt, "counters": info.Counters, "statistics": stats, "stateSchemaVersion": SchemaVersion, "manifestSchemaVersion": 1, "manifestOptions": options}
	return writeJSONFile(filepath.Join(store.Root(), "summary.json"), summary)
}
func statistics(store *Store) map[string]any {
	store.mu.RLock()
	defer store.mu.RUnlock()
	widths, heights := []int{}, []int{}
	titleIDs, chapterIDs, hashes := map[string]bool{}, map[string]bool{}, map[string]map[string]bool{}
	rejection, failures := map[string]int{}, map[string]int{}
	var bytes int64
	retries := 0
	for _, attempt := range store.data.Attempts {
		if attempt.Attempt > 1 {
			retries += attempt.Attempt - 1
		}
	}
	for _, page := range store.data.Pages {
		if page.State == "valid" {
			widths = append(widths, page.Width)
			heights = append(heights, page.Height)
			titleIDs[page.TitleID] = true
			chapterIDs[page.ChapterID] = true
			bytes += page.Bytes
			if page.SHA256 != "" {
				if hashes[page.SHA256] == nil {
					hashes[page.SHA256] = map[string]bool{}
				}
				if title, ok := store.data.Titles[page.TitleID]; ok && title.Split != "" {
					hashes[page.SHA256][title.Split] = true
				}
			}
		}
		if page.State == "rejected" {
			rejection[page.RejectionCode]++
		}
	}
	for _, chapter := range store.data.Chapters {
		if chapter.State == "failed" {
			failures[chapter.LastError]++
		}
	}
	avg := func(values []int) float64 {
		if len(values) == 0 {
			return 0
		}
		total := 0
		for _, v := range values {
			total += v
		}
		return float64(total) / float64(len(values))
	}
	minmax := func(values []int) (int, int) {
		if len(values) == 0 {
			return 0, 0
		}
		min, max := values[0], values[0]
		for _, v := range values {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		return min, max
	}
	minW, maxW := minmax(widths)
	minH, maxH := minmax(heights)
	return map[string]any{"splitCounts": splitCounts(store.data.Titles), "rejectionCategories": rejection, "failureCategories": failures, "retryCount": retries, "archiveBytes": int64(0), "storedBytes": bytes, "width": map[string]any{"average": avg(widths), "minimum": minW, "maximum": maxW}, "height": map[string]any{"average": avg(heights), "minimum": minH, "maximum": maxH}, "pagesPerTitle": ratio(len(widths), len(titleIDs)), "pagesPerChapter": ratio(len(widths), len(chapterIDs))}
}
func splitCounts(titles map[string]Title) map[string]int {
	counts := map[string]int{}
	for _, title := range titles {
		if title.State != "discovered" {
			counts[title.Split]++
		}
	}
	return counts
}
func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}
func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
	store.mu.RLock()
	pages := make([]Page, 0, len(store.data.Pages))
	for _, page := range store.data.Pages {
		pages = append(pages, page)
	}
	store.mu.RUnlock()
	checked, invalid := 0, 0
	for _, page := range pages {
		if page.State != "valid" {
			continue
		}
		checked++
		path := filepath.Join(store.Root(), filepath.FromSlash(page.RelativePath))
		validated, _, err := ValidateFile(path, info.Config.Validation)
		bad := err != nil || (page.SHA256 != "" && validated.SHA256 != page.SHA256) || !matchesOutputFormat(path, validated.MIMEType, info.Config.Output.Format)
		if bad {
			invalid++
			if repair {
				if err := store.setPagePending(page.ChapterID, page.Index, "file missing or modified"); err != nil {
					return nil, err
				}
			}
		}
	}
	if repair {
		if err := store.repairState(); err != nil {
			return nil, err
		}
		for _, path := range mustTemporaryFiles(store.Root()) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return nil, err
			}
		}
		for _, path := range mustStaging(store.Root()) {
			if err := os.RemoveAll(path); err != nil {
				return nil, err
			}
		}
		if err := Export(ctx, store, ExportOptions{}); err != nil {
			return nil, err
		}
	}
	state := store.stateInconsistencies()
	duplicates := store.invalidDuplicates()
	manifest, err := verifyManifest(store, info)
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
	unexpected, err := unexpectedDataFiles(store)
	if err != nil {
		return nil, err
	}
	leakage := store.splitLeakage()
	return map[string]any{"datasetRoot": store.Root(), "checkedPages": checked, "invalidPages": invalid, "adoptedArchives": 0, "stateInconsistencies": state, "invalidDuplicateReferences": duplicates, "manifestInconsistencies": manifest, "temporaryFiles": len(temporary), "stagingDirectories": len(staging), "unexpectedFiles": len(unexpected), "splitLeakage": leakage, "repair": repair, "valid": invalid == 0 && state == 0 && duplicates == 0 && manifest == 0 && leakage == 0 && len(temporary) == 0 && len(staging) == 0 && len(unexpected) == 0}, nil
}
func matchesOutputFormat(path, mime string, format archive.Format) bool {
	switch format {
	case archive.FormatPNG:
		return strings.EqualFold(filepath.Ext(path), ".png") && mime == "image/png"
	case archive.FormatJPEG:
		return strings.EqualFold(filepath.Ext(path), ".jpeg") && mime == "image/jpeg"
	case archive.FormatDirectory:
		return true
	}
	return false
}
func (s *Store) setPagePending(chapter string, index int, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := pageKey(chapter, index)
	page := s.data.Pages[key]
	page.State, page.ErrorMessage = "pending", message
	s.data.Pages[key] = page
	return s.saveLocked()
}
func (s *Store) repairState() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, chapter := range s.data.Chapters {
		valid := 0
		for _, page := range s.data.Pages {
			if page.ChapterID == id && page.State == "valid" {
				valid++
			}
		}
		if chapter.State == "downloading" || valid < chapter.ExpectedPages {
			chapter.State = "partial"
			chapter.ClaimOwner = ""
			if chapter.LastError == "" {
				chapter.LastError = "incomplete chapter"
			}
			s.data.Chapters[id] = chapter
		}
		s.refreshTitleLocked(chapter.TitleID)
	}
	return s.saveLocked()
}
func (s *Store) stateInconsistencies() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bad := 0
	for _, chapter := range s.data.Chapters {
		valid := 0
		for _, page := range s.data.Pages {
			if page.ChapterID == chapter.ID && page.State == "valid" {
				valid++
			}
		}
		if chapter.State == "completed" && chapter.ExpectedPages > 0 && valid != chapter.ExpectedPages {
			bad++
		}
	}
	for _, title := range s.data.Titles {
		if title.State != "completed" {
			continue
		}
		for _, chapter := range s.data.Chapters {
			if chapter.TitleID == title.ID && chapter.State != "completed" {
				bad++
				break
			}
		}
	}
	return bad
}
func (s *Store) invalidDuplicates() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bad := 0
	for _, page := range s.data.Pages {
		if page.State != "valid" {
			continue
		}
		for _, ref := range []string{page.ExactDuplicateOf, page.NearDuplicateOf} {
			if ref == "" {
				continue
			}
			parts := strings.Split(ref, ":")
			if len(parts) != 2 {
				bad++
				continue
			}
			found := false
			for _, candidate := range s.data.Pages {
				if candidate.ChapterID == parts[0] && fmt.Sprint(candidate.Index) == parts[1] && candidate.State == "valid" {
					found = true
				}
			}
			if !found {
				bad++
			}
		}
	}
	return bad
}
func (s *Store) splitLeakage() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hashes := map[string]map[string]bool{}
	for _, page := range s.data.Pages {
		title := s.data.Titles[page.TitleID]
		if page.State == "valid" && page.SHA256 != "" && title.Split != "" {
			if hashes[page.SHA256] == nil {
				hashes[page.SHA256] = map[string]bool{}
			}
			hashes[page.SHA256][title.Split] = true
		}
	}
	n := 0
	for _, splits := range hashes {
		if len(splits) > 1 {
			n++
		}
	}
	return n
}
func verifyManifest(store *Store, info DatasetInfo) (int, error) {
	options, err := manifestExportOptions(store.Root())
	if err != nil {
		return 0, err
	}
	want, err := manifestRecords(store, options)
	if err != nil {
		return 0, err
	}
	file, err := os.Open(filepath.Join(store.Root(), "manifest.jsonl"))
	if os.IsNotExist(err) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()
	got := []ManifestRecord{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var r ManifestRecord
		if json.Unmarshal(scanner.Bytes(), &r) != nil {
			return 1, nil
		}
		got = append(got, r)
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if len(want) != len(got) {
		return abs(len(want) - len(got)), nil
	}
	for i := range want {
		if want[i] != got[i] {
			return 1, nil
		}
	}
	return 0, nil
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
		return ExportOptions{}, err
	}
	return summary.ManifestOptions, nil
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func temporaryFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".part") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
func mustTemporaryFiles(root string) []string { files, _ := temporaryFiles(root); return files }
func archiveStagingDirectories(root string) ([]string, error) {
	base := filepath.Join(root, ".staging")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, nil
	}
	paths := map[string]bool{}
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths[filepath.Dir(path)] = true
		}
		return nil
	})
	out := []string{}
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, err
}
func mustStaging(root string) []string { paths, _ := archiveStagingDirectories(root); return paths }
func unexpectedDataFiles(store *Store) ([]string, error) {
	store.mu.RLock()
	expected := map[string]bool{}
	for _, page := range store.data.Pages {
		if page.State == "valid" && page.RelativePath != "" {
			expected[filepath.ToSlash(page.RelativePath)] = true
		}
	}
	store.mu.RUnlock()
	root := filepath.Join(store.Root(), "data")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}
	bad := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
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
		if !expected[filepath.ToSlash(relative)] {
			bad = append(bad, filepath.ToSlash(relative))
		}
		return nil
	})
	return bad, err
}
func Failures(_ context.Context, store *Store) error {
	store.mu.RLock()
	defer store.mu.RUnlock()
	path := filepath.Join(store.Root(), "reports", "failures.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".failures-*")
	if err != nil {
		return err
	}
	w := bufio.NewWriter(tmp)
	ids := make([]string, 0, len(store.data.Chapters))
	for id := range store.data.Chapters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		chapter := store.data.Chapters[id]
		if chapter.State == "failed" {
			data, _ := json.Marshal(map[string]any{"entityType": "chapter", "chapterId": id, "state": chapter.State, "code": "", "message": strings.TrimSpace(chapter.LastError)})
			_, _ = w.Write(append(data, '\n'))
		}
	}
	for _, page := range store.data.Pages {
		if page.State == "rejected" || page.State == "failed" {
			data, _ := json.Marshal(map[string]any{"entityType": "page", "chapterId": page.ChapterID, "pageIndex": page.Index, "state": page.State, "code": page.RejectionCode, "message": strings.TrimSpace(page.ErrorMessage)})
			_, _ = w.Write(append(data, '\n'))
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
