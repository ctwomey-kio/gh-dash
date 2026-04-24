package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestStore() *NotifiedStore {
	// filePath="" means no persistence — pure in-memory for tests.
	return &NotifiedStore{entries: make(map[string]time.Time)}
}

func TestIsNotifiedMissOnEmpty(t *testing.T) {
	s := newTestStore()
	require.False(t, s.IsNotified("notif-1", time.Now()))
}

func TestMarkAndIsNotified(t *testing.T) {
	s := newTestStore()
	now := time.Now().Truncate(time.Second)

	s.entries["notif-1"] = now // bypass save() by writing directly

	require.True(t, s.IsNotified("notif-1", now))
	require.True(t, s.IsNotified("notif-1", now.Add(-1*time.Minute)), "older updatedAt should still be notified")
	require.False(t, s.IsNotified("notif-1", now.Add(1*time.Minute)), "newer updatedAt means new activity → should fire again")
}

func TestSeedIfNeededFirstLaunch(t *testing.T) {
	s := newTestStore()
	now := time.Now().Truncate(time.Second)

	ids := []string{"a", "b", "c"}
	updatedAts := []time.Time{now, now.Add(-1 * time.Hour), now.Add(-2 * time.Hour)}

	seeded := s.SeedIfNeeded(ids, updatedAts)
	require.True(t, seeded, "first call with empty store should seed and return true")

	for i, id := range ids {
		require.True(t, s.IsNotified(id, updatedAts[i]))
	}
}

func TestSeedIfNeededNoopOnSubsequentCall(t *testing.T) {
	s := newTestStore()
	now := time.Now()

	s.SeedIfNeeded([]string{"x"}, []time.Time{now})
	seeded := s.SeedIfNeeded([]string{"y"}, []time.Time{now})
	require.False(t, seeded, "second call should not seed again")
	require.False(t, s.IsNotified("y", now), "y should not have been added")
}

func TestSeedIfNeededNoopWhenStoreAlreadyHasEntries(t *testing.T) {
	s := newTestStore()
	now := time.Now()

	s.entries["existing"] = now

	seeded := s.SeedIfNeeded([]string{"new"}, []time.Time{now})
	require.False(t, seeded, "non-empty store should skip seeding")
	require.False(t, s.IsNotified("new", now))
}

func TestPruneRemovesOldEntries(t *testing.T) {
	s := newTestStore()

	old := time.Now().Add(-100 * 24 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)

	s.entries["old-notif"] = old
	s.entries["recent-notif"] = recent
	s.prune()

	require.NotContains(t, s.entries, "old-notif")
	require.Contains(t, s.entries, "recent-notif")
}
