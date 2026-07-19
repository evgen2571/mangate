package dataset

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/evgen2571/mangate/internal/archive"
	"github.com/evgen2571/mangate/internal/downloader"
	"github.com/evgen2571/mangate/internal/providers"
	"github.com/evgen2571/mangate/internal/source"
)

type Service struct {
	Store      *Store
	Provider   providers.Provider
	Downloader *downloader.Downloader
}
type CollectResult struct {
	DatasetRoot    string         `json:"datasetRoot"`
	DatasetID      string         `json:"datasetId"`
	Provider       string         `json:"provider"`
	Format         archive.Format `json:"format"`
	State          string         `json:"state"`
	StoppingReason string         `json:"stoppingReason"`
	Counters       Counters       `json:"counters"`
	ManifestPath   string         `json:"manifestPath"`
	SummaryPath    string         `json:"summaryPath"`
	Resumed        bool           `json:"resumed"`
}
type limitReachedError struct{ reason string }

func (e limitReachedError) Error() string { return e.reason }

type validatedPage struct {
	result downloader.PageDownloadResult
	image  ValidatedImage
	code   string
	err    error
}

func (s Service) Collect(ctx context.Context, cfg Config, resume bool) (CollectResult, error) {
	if err := s.Store.Initialize(ctx, cfg, resume); err != nil {
		return CollectResult{}, err
	}
	if resume {
		if _, err := Verify(ctx, s.Store, true); err != nil {
			return CollectResult{}, fmt.Errorf("reconcile resumed dataset: %w", err)
		}
	}
	titles, chapters, err := s.Store.Planned(ctx)
	if err != nil {
		return CollectResult{}, err
	}
	if len(titles) == 0 {
		if _, err := BuildPlan(ctx, s.Store, s.Provider, cfg); err != nil {
			return CollectResult{}, err
		}
		titles, chapters, err = s.Store.Planned(ctx)
		if err != nil {
			return CollectResult{}, err
		}
	}
	if err := s.Store.SetRun(ctx, "collecting", "", ""); err != nil {
		return CollectResult{}, err
	}
	titleByID := map[string]Title{}
	for _, title := range titles {
		titleByID[title.ID] = title
	}
	claimOwner := "collector-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	reason, err := s.collectChapters(ctx, cfg, chapters, titleByID, claimOwner)
	if err != nil {
		if interrupted := ctx.Err(); interrupted != nil {
			return s.interrupted(interrupted)
		}
		return CollectResult{}, err
	}
	state := "completed"
	info, _ := s.Store.Info(ctx)
	if info.Counters.FailedChapters > 0 {
		state = "partial"
	}
	if reason != "" {
		state = "partial"
	}
	if err := s.Store.SetRun(ctx, state, reason, ""); err != nil {
		return CollectResult{}, err
	}
	if err := Export(ctx, s.Store, ExportOptions{}); err != nil {
		return CollectResult{}, err
	}
	if err := Failures(ctx, s.Store); err != nil {
		return CollectResult{}, err
	}
	return s.result(ctx, cfg, resume, state, reason)
}

// collectChapters runs independent chapter transfers through a bounded worker
// pool. Claims and limit checks are serialized so a single run never starts
// the same chapter twice; downloading and validation remain concurrent.
func (s Service) collectChapters(ctx context.Context, cfg Config, chapters []Chapter, titleByID map[string]Title, claimOwner string) (string, error) {
	workers := cfg.Runtime.ChapterWorkers
	if workers > len(chapters) {
		workers = len(chapters)
	}
	if workers == 0 {
		return "", nil
	}
	jobs := make(chan Chapter)
	var workersDone sync.WaitGroup
	var stateMu sync.Mutex
	var commitMu sync.Mutex
	reason := ""
	var runErr error

	stopped := func() bool {
		stateMu.Lock()
		defer stateMu.Unlock()
		return reason != "" || runErr != nil
	}
	setReason := func(value string) {
		stateMu.Lock()
		if reason == "" {
			reason = value
		}
		stateMu.Unlock()
	}
	setError := func(err error) {
		stateMu.Lock()
		if runErr == nil {
			runErr = err
		}
		stateMu.Unlock()
	}

	for range workers {
		workersDone.Add(1)
		go func() {
			defer workersDone.Done()
			for chapterRecord := range jobs {
				if ctx.Err() != nil || stopped() || chapterRecord.State == "completed" {
					continue
				}
				stateMu.Lock()
				if reason != "" || runErr != nil {
					stateMu.Unlock()
					continue
				}
				info, err := s.Store.Info(ctx)
				if err != nil {
					runErr = err
					stateMu.Unlock()
					continue
				}
				if limit := collectionLimit(cfg, info.Counters, chapterRecord.ExpectedPages); limit != "" {
					reason = limit
					stateMu.Unlock()
					continue
				}
				claimed, err := s.Store.ClaimChapter(ctx, chapterRecord.ID, claimOwner)
				stateMu.Unlock()
				if err != nil {
					setError(fmt.Errorf("claim chapter %q: %w", chapterRecord.ID, err))
					continue
				}
				if !claimed {
					continue
				}
				title, ok := titleByID[chapterRecord.TitleID]
				if !ok {
					setError(fmt.Errorf("planned chapter %q references unknown title %q", chapterRecord.ID, chapterRecord.TitleID))
					continue
				}
				manga := &source.Manga{ID: title.ID, Title: title.Name, URL: title.URL, Metadata: source.MangaMetadata{Language: title.OriginalLanguage, Status: title.Status, ContentType: title.ContentRating, Year: title.Year}}
				chapter := &source.Chapter{ID: chapterRecord.ID, Index: chapterRecord.Number, Title: chapterRecord.Name, Volume: chapterRecord.Volume, Language: chapterRecord.Language, ReleaseGroup: chapterRecord.ReleaseGroup, PublishedAt: chapterRecord.PublishedAt, URL: chapterRecord.URL, PageCount: chapterRecord.ExpectedPages, From: manga}
				if err := s.collectChapter(ctx, cfg, chapter, title, &commitMu); err != nil {
					if ctx.Err() != nil {
						continue
					}
					var limitErr limitReachedError
					if errors.As(err, &limitErr) {
						setReason(limitErr.reason)
						continue
					}
					commitMu.Lock()
					failErr := s.Store.FailChapter(ctx, chapter.ID, err.Error())
					if failErr != nil {
						commitMu.Unlock()
						setError(fmt.Errorf("fail chapter %q: %w", chapter.ID, failErr))
						continue
					}
					info, infoErr := s.Store.Info(ctx)
					commitMu.Unlock()
					if infoErr != nil {
						setError(infoErr)
						continue
					}
					if value := collectionLimit(cfg, info.Counters, 0); value != "" {
						setReason(value)
					}
				}
			}
		}()
	}
	for _, chapter := range chapters {
		select {
		case jobs <- chapter:
		case <-ctx.Done():
			close(jobs)
			workersDone.Wait()
			return "", ctx.Err()
		}
	}
	close(jobs)
	workersDone.Wait()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	stateMu.Lock()
	defer stateMu.Unlock()
	return reason, runErr
}

func collectionLimit(cfg Config, counters Counters, expectedPages int) string {
	if cfg.Limits.MaxPages > 0 && int64(counters.ValidPages)+int64(expectedPages) > cfg.Limits.MaxPages {
		return "max_pages"
	}
	if cfg.Limits.MaxBytes > 0 && counters.StoredBytes >= cfg.Limits.MaxBytes {
		return "max_bytes"
	}
	if cfg.Limits.MaxFailures > 0 && counters.FailedChapters >= cfg.Limits.MaxFailures {
		return "max_failures"
	}
	return ""
}

func (s Service) interrupted(err error) (CollectResult, error) {
	if err == nil {
		err = context.Canceled
	}
	_ = s.Store.SetRun(context.Background(), "interrupted", "interrupted", err.Error())
	return CollectResult{}, err
}

// validateDownloadedPages bounds full image decoding and hashing so a chapter
// cannot allocate validation work proportional to its page count.
func validateDownloadedPages(ctx context.Context, cfg Validation, workers int, downloaded []downloader.PageDownloadResult) ([]validatedPage, error) {
	if len(downloaded) == 0 {
		return nil, nil
	}
	if workers > len(downloaded) {
		workers = len(downloaded)
	}
	pages := make([]validatedPage, len(downloaded))
	jobs := make(chan int)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				result := downloaded[index]
				record, code, err := ValidateFile(result.Path, cfg)
				pages[index] = validatedPage{result: result, image: record, code: code, err: err}
			}
		}()
	}
	for index := range downloaded {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			group.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	group.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return pages, nil
}

func (s Service) collectChapter(ctx context.Context, cfg Config, chapter *source.Chapter, titleRecord Title, commitMu *sync.Mutex) error {
	split := titleRecord.Split
	titleDir := filepath.Join(s.Store.Root(), "data", normalizedTitle(chapter.From.Title))
	if err := os.MkdirAll(titleDir, 0o755); err != nil {
		return fmt.Errorf("create title directory: %w", err)
	}
	chapterDir := filepath.Join(titleDir, "chapter-"+normalizedChapterNumber(chapter.Index))
	results, err := s.Downloader.DownloadChapterTo(ctx, chapter, chapterDir, s.Provider.Pages)
	if err != nil {
		return err
	}
	// Dataset state lives in dataset-state.json. Keep chapter directories to
	// ordered page files so they remain straightforward to browse or copy.
	if err := os.Remove(filepath.Join(chapterDir, ".mangate.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove downloader state: %w", err)
	}
	validated, err := validateDownloadedPages(ctx, cfg.Validation, cfg.Runtime.ValidationWorkers, results)
	if err != nil {
		return err
	}
	commitMu.Lock()
	defer commitMu.Unlock()
	valid := true
	accepted := make([]validatedPage, 0, len(validated))
	for _, page := range validated {
		if page.err != nil {
			valid = false
			_ = s.Store.RecordPage(ctx, Page{TitleID: chapter.From.ID, ChapterID: chapter.ID, Index: page.result.PageIndex, State: "rejected", RejectionCode: page.code, ErrorMessage: page.err.Error(), Split: split})
			continue
		}
		accepted = append(accepted, page)
	}
	if !valid {
		return s.Store.CompleteChapter(ctx, chapter.ID, chapterDir, false, "one or more pages failed validation")
	}
	info, err := s.Store.Info(ctx)
	if err != nil {
		return err
	}
	chapterBytes := int64(0)
	for _, item := range accepted {
		chapterBytes += item.image.Bytes
	}
	limitReason := ""
	if cfg.Limits.MaxPages > 0 && int64(info.Counters.ValidPages)+int64(len(accepted)) > cfg.Limits.MaxPages {
		limitReason = "max_pages"
	}
	if limitReason == "" && cfg.Limits.MaxBytes > 0 && info.Counters.StoredBytes+chapterBytes > cfg.Limits.MaxBytes {
		limitReason = "max_bytes"
	}
	if limitReason != "" {
		for _, item := range accepted {
			_ = s.Store.RecordPage(ctx, Page{TitleID: chapter.From.ID, ChapterID: chapter.ID, Index: item.result.PageIndex, State: "rejected", RejectionCode: limitReason, ErrorMessage: "collection limit reached", Split: split})
		}
		if err := s.Store.CompleteChapter(ctx, chapter.ID, chapterDir, false, "collection limit reached"); err != nil {
			return err
		}
		allNew := true
		for _, item := range accepted {
			if item.result.Reused {
				allNew = false
				break
			}
		}
		if allNew {
			_ = os.RemoveAll(chapterDir)
		}
		return limitReachedError{reason: limitReason}
	}
	for _, item := range accepted {
		result, record := item.result, item.image
		relative, err := filepath.Rel(s.Store.Root(), result.Path)
		if err != nil {
			return err
		}
		duplicate := ""
		if record.SHA256 != "" {
			duplicate, _ = s.findDuplicate(ctx, record.SHA256, chapter.ID, result.PageIndex)
		}
		nearDuplicate := ""
		if duplicate == "" && record.PerceptualHash != "" {
			nearDuplicate, _ = s.findNearDuplicate(ctx, record.PerceptualHash, chapter.ID, result.PageIndex)
		}
		if err := s.Store.RecordPage(ctx, Page{TitleID: chapter.From.ID, ChapterID: chapter.ID, Index: result.PageIndex, StorageType: "file", RelativePath: filepath.ToSlash(relative), SourceMIMEType: result.SourceContentType, MIMEType: record.MIMEType, Extension: result.Extension, Width: record.Width, Height: record.Height, Bytes: record.Bytes, SHA256: record.SHA256, PerceptualHash: record.PerceptualHash, ExactDuplicateOf: duplicate, NearDuplicateOf: nearDuplicate, State: "valid", Split: split}); err != nil {
			return err
		}
	}
	if err := s.Store.CompleteChapter(ctx, chapter.ID, chapterDir, valid, ""); err != nil {
		return err
	}
	return nil
}
func (s Service) findNearDuplicate(ctx context.Context, perceptualHash, chapterID string, index int) (string, error) {
	want, err := strconv.ParseUint(perceptualHash, 16, 64)
	if err != nil {
		return "", err
	}
	s.Store.mu.RLock()
	defer s.Store.mu.RUnlock()
	for _, candidatePage := range s.Store.data.Pages {
		if candidatePage.State != "valid" || candidatePage.PerceptualHash == "" || (candidatePage.ChapterID == chapterID && candidatePage.Index == index) {
			continue
		}
		id, page, hash := candidatePage.ChapterID, candidatePage.Index, candidatePage.PerceptualHash
		candidate, err := strconv.ParseUint(hash, 16, 64)
		if err == nil && bits.OnesCount64(want^candidate) <= 4 {
			return id + ":" + strconv.Itoa(page), nil
		}
	}
	return "", nil
}
func (s Service) findDuplicate(ctx context.Context, hash, chapterID string, index int) (string, error) {
	s.Store.mu.RLock()
	defer s.Store.mu.RUnlock()
	identities := []string{}
	for _, page := range s.Store.data.Pages {
		if page.State == "valid" && page.SHA256 == hash && !(page.ChapterID == chapterID && page.Index == index) {
			identities = append(identities, page.ChapterID+":"+strconv.Itoa(page.Index))
		}
	}
	if len(identities) == 0 {
		return "", nil
	}
	sort.Strings(identities)
	return identities[0], nil
}
func (s Service) result(ctx context.Context, cfg Config, resume bool, state, reason string) (CollectResult, error) {
	info, err := s.Store.Info(ctx)
	if err != nil {
		return CollectResult{}, err
	}
	return CollectResult{DatasetRoot: s.Store.Root(), DatasetID: cfg.DatasetID, Provider: cfg.Provider, Format: cfg.Output.Format, State: state, StoppingReason: reason, Counters: info.Counters, ManifestPath: filepath.Join(s.Store.Root(), "manifest.jsonl"), SummaryPath: filepath.Join(s.Store.Root(), "summary.json"), Resumed: resume}, nil
}
func normalizedTitle(value string) string         { return normalizedSegment(value, false) }
func normalizedChapterNumber(value string) string { return normalizedSegment(value, true) }

func normalizedSegment(value string, allowDot bool) string {
	var out strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || (allowDot && r == '.') {
			out.WriteRune(r)
			dash = false
			continue
		}
		if !dash && out.Len() > 0 {
			out.WriteByte('-')
			dash = true
		}
	}
	if normalized := strings.Trim(out.String(), "-."); normalized != "" {
		return normalized
	}
	return "unknown"
}
