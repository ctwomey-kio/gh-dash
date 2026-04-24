package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"charm.land/log/v2"
)

// NotifiedStore persists notification IDs along with the timestamp at which a
// desktop notification was last fired for them. When checking whether to fire
// again we compare the stored timestamp against the notification's current
// updated_at: if the notification has been updated since it was last notified,
// we fire again (same resurface-on-update semantics as DoneStore).
type NotifiedStore struct {
	mu       sync.RWMutex
	entries  map[string]time.Time // id -> updatedAt when last desktop-notified
	filePath string
	seeded   bool
}

func newNotifiedStore(filename string) *NotifiedStore {
	store := &NotifiedStore{
		entries: make(map[string]time.Time),
	}
	filePath, err := getStateFilePath(filename)
	if err != nil {
		log.Error("Failed to get state file path for notified store", "err", err)
	}
	store.filePath = filePath
	if err := store.load(); err != nil {
		log.Error("Failed to load notified store", "err", err)
	}
	return store
}

func (s *NotifiedStore) load() error {
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
	for id, raw := range tsMap {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			log.Warn("Skipping notified entry with invalid timestamp", "id", id, "raw", raw, "err", err)
			continue
		}
		s.entries[id] = t
	}
	s.prune()
	log.Debug("Loaded notified store", "count", len(s.entries))
	return nil
}

// prune removes entries older than 90 days.
func (s *NotifiedStore) prune() {
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	for id, t := range s.entries {
		if t.IsZero() || t.Before(cutoff) {
			delete(s.entries, id)
		}
	}
}

func (s *NotifiedStore) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.filePath == "" {
		return nil
	}

	tsMap := make(map[string]string, len(s.entries))
	for id, t := range s.entries {
		tsMap[id] = t.Format(time.RFC3339)
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

	log.Debug("Saved notified store", "count", len(tsMap))
	return nil
}

// MarkNotified records the notification's current updated_at as the
// "last notified at" timestamp. If the notification later receives new
// activity (a newer updated_at), IsNotified will return false.
func (s *NotifiedStore) MarkNotified(id string, updatedAt time.Time) {
	s.mu.Lock()
	s.entries[id] = updatedAt
	s.mu.Unlock()
	go s.save()
}

// IsNotified returns true only if the notification has not been updated
// since it was last desktop-notified: !updatedAt.After(notifiedAt).
func (s *NotifiedStore) IsNotified(id string, updatedAt time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	notifiedAt, ok := s.entries[id]
	if !ok {
		return false
	}
	return !updatedAt.After(notifiedAt)
}

// SeedIfNeeded marks all provided notification IDs as notified on first launch
// (when the store is empty / no file exists) to prevent a notification flood.
// Returns true if seeding occurred (caller should skip firing notifications).
func (s *NotifiedStore) SeedIfNeeded(ids []string, updatedAts []time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.seeded || len(s.entries) > 0 {
		s.seeded = true
		return false
	}

	for i, id := range ids {
		var t time.Time
		if i < len(updatedAts) {
			t = updatedAts[i]
		} else {
			t = time.Now()
		}
		s.entries[id] = t
	}
	s.seeded = true
	go s.save()
	log.Debug("Seeded notified store on first launch", "count", len(s.entries))
	return true
}

// Flush forces an immediate synchronous save.
func (s *NotifiedStore) Flush() error {
	return s.save()
}

// Singleton

var (
	notifiedStore     *NotifiedStore
	notifiedStoreOnce sync.Once
)

// GetNotifiedStore returns the singleton notified store.
func GetNotifiedStore() *NotifiedStore {
	notifiedStoreOnce.Do(func() {
		notifiedStore = newNotifiedStore("notified.json")
	})
	return notifiedStore
}
