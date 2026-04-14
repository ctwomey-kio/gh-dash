package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"charm.land/log/v2"
)

// SeenStore persists PR URLs that have already triggered desktop notifications.
// On first launch (empty store), all current PRs are seeded as seen to avoid
// a notification flood. Subsequent fetches only notify for genuinely new PRs.
type SeenStore struct {
	mu       sync.RWMutex
	entries  map[string]time.Time // PR URL -> time first seen
	filePath string
	seeded   bool
}

func newSeenStore() *SeenStore {
	store := &SeenStore{
		entries: make(map[string]time.Time),
	}
	filePath, err := getStateFilePath("seen-prs.json")
	if err != nil {
		log.Error("Failed to get state file path for seen PRs", "err", err)
	}
	store.filePath = filePath
	if err := store.load(); err != nil {
		log.Error("Failed to load seen PRs", "err", err)
	}
	return store
}

func (s *SeenStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tsMap map[string]string
	if err := json.Unmarshal(data, &tsMap); err != nil {
		return err
	}
	for url, raw := range tsMap {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			continue
		}
		s.entries[url] = t
	}
	s.prune()
	s.seeded = len(s.entries) > 0
	log.Debug("Loaded seen PRs", "count", len(s.entries))
	return nil
}

func (s *SeenStore) prune() {
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	for url, t := range s.entries {
		if t.Before(cutoff) {
			delete(s.entries, url)
		}
	}
}

func (s *SeenStore) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.filePath == "" {
		return nil
	}

	tsMap := make(map[string]string, len(s.entries))
	for url, t := range s.entries {
		tsMap[url] = t.Format(time.RFC3339)
	}

	data, err := json.Marshal(tsMap)
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	log.Debug("Saved seen PRs", "count", len(tsMap))
	return nil
}

// SeedIfNeeded marks all provided URLs as seen on the very first call when
// the store is empty (fresh install or cleared state). Returns true when
// seeding occurred — callers should suppress notifications in that case.
func (s *SeenStore) SeedIfNeeded(urls []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.seeded {
		return false
	}

	now := time.Now()
	for _, url := range urls {
		s.entries[url] = now
	}
	s.seeded = true
	go s.save()
	log.Debug("Seeded seen PRs store", "count", len(urls))
	return true
}

// IsSeen returns true if the PR URL has been seen before.
func (s *SeenStore) IsSeen(url string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.entries[url]
	return ok
}

// MarkSeen records a PR URL as seen.
func (s *SeenStore) MarkSeen(url string) {
	s.mu.Lock()
	s.entries[url] = time.Now()
	s.mu.Unlock()
	go s.save()
}

// Flush forces an immediate synchronous save.
func (s *SeenStore) Flush() error {
	return s.save()
}

// Singleton

var (
	seenStore     *SeenStore
	seenStoreOnce sync.Once
)

// GetSeenStore returns the singleton seen PR store.
func GetSeenStore() *SeenStore {
	seenStoreOnce.Do(func() {
		seenStore = newSeenStore()
	})
	return seenStore
}
