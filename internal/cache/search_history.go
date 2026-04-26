package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evgen2571/mangate/internal/util"
)

const searchHistoryFileName = "search-history.json"

type searchHistoryFile struct {
	Queries []string `json:"queries"`
}

func (c *Cache) SearchHistory() ([]string, error) {
	if c.cfg.Search.HistoryMax <= 0 {
		return nil, nil
	}

	queries, err := c.loadSearchHistory()
	if err != nil {
		return nil, err
	}
	return limitSearchHistory(queries, c.cfg.Search.HistoryMax), nil
}

func (c *Cache) AddSearchQuery(query string) error {
	query = strings.TrimSpace(query)
	if query == "" || c.cfg.Search.HistoryMax <= 0 {
		return nil
	}

	queries, err := c.loadSearchHistory()
	if err != nil {
		return err
	}
	queries = prependSearchQuery(queries, query, c.cfg.Search.HistoryMax)
	return c.saveSearchHistory(queries)
}

func (c *Cache) loadSearchHistory() ([]string, error) {
	path := c.searchHistoryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read search history %q: %w", path, err)
	}

	var file searchHistoryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode search history %q: %w", path, err)
	}
	return cleanSearchHistory(file.Queries), nil
}

func (c *Cache) saveSearchHistory(queries []string) error {
	path := c.searchHistoryPath()
	if err := util.EnsureDir(filepath.Dir(path), "search history cache directory"); err != nil {
		return err
	}

	data, err := json.MarshalIndent(searchHistoryFile{Queries: queries}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode search history %q: %w", path, err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), "search-history-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary search history file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary search history file %q: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary search history file %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace search history %q: %w", path, err)
	}
	return nil
}

func (c *Cache) searchHistoryPath() string {
	return filepath.Join(c.cfg.Dirs.Cache, searchHistoryFileName)
}

func prependSearchQuery(queries []string, query string, max int) []string {
	query = strings.TrimSpace(query)
	if query == "" || max <= 0 {
		return nil
	}

	result := []string{query}
	seen := map[string]struct{}{strings.ToLower(query): {}}
	for _, existing := range queries {
		existing = strings.TrimSpace(existing)
		if existing == "" {
			continue
		}
		key := strings.ToLower(existing)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, existing)
		if len(result) == max {
			break
		}
	}
	return result
}

func cleanSearchHistory(queries []string) []string {
	result := make([]string, 0, len(queries))
	seen := make(map[string]struct{}, len(queries))
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		key := strings.ToLower(query)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, query)
	}
	return result
}

func limitSearchHistory(queries []string, max int) []string {
	if max <= 0 || len(queries) == 0 {
		return nil
	}
	queries = cleanSearchHistory(queries)
	if len(queries) > max {
		queries = queries[:max]
	}
	return queries
}
