package ai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ── Cache ──────────────────────────────────────────────────────────────────────

func TestCacheGetMissAndHit(t *testing.T) {
	c := NewCache[PRSummaryResponse](10)

	key := CacheKey{URL: "https://github.com/org/repo/pull/1", UpdatedAt: time.Now()}
	_, ok := c.Get(key)
	require.False(t, ok, "expect miss on empty cache")

	want := PRSummaryResponse{Category: "bug", Summary: "fixes nil ptr"}
	c.Set(key, want)

	got, ok := c.Get(key)
	require.True(t, ok, "expect hit after Set")
	require.Equal(t, want, got)
}

func TestCacheEvictsOldestOnOverflow(t *testing.T) {
	c := NewCache[PRSummaryResponse](2)

	k1 := CacheKey{URL: "u1"}
	k2 := CacheKey{URL: "u2"}
	k3 := CacheKey{URL: "u3"}

	c.Set(k1, PRSummaryResponse{Summary: "one"})
	c.Set(k2, PRSummaryResponse{Summary: "two"})
	c.Set(k3, PRSummaryResponse{Summary: "three"})

	_, ok := c.Get(k1)
	require.False(t, ok, "k1 should be evicted (oldest)")
	_, ok = c.Get(k2)
	require.True(t, ok)
	_, ok = c.Get(k3)
	require.True(t, ok)
}

func TestCacheKeyInvalidatesOnViewerStateChange(t *testing.T) {
	c := NewCache[PRSummaryResponse](10)
	updatedAt := time.Now()

	k1 := CacheKey{URL: "u", UpdatedAt: updatedAt, ViewerReviewState: ""}
	k2 := CacheKey{URL: "u", UpdatedAt: updatedAt, ViewerReviewState: "APPROVED"}

	c.Set(k1, PRSummaryResponse{Summary: "before approve"})

	_, ok := c.Get(k2)
	require.False(t, ok, "different ViewerReviewState should be a cache miss")
}

// ── ParsePRSummary ─────────────────────────────────────────────────────────────

func TestParsePRSummaryHappyPath(t *testing.T) {
	raw := `{"category":"feature","interest":"high","summary":"adds AI tab","key_files":["prview.go"],"review_status":"@alice approved"}`
	got, err := ParsePRSummary(raw)
	require.NoError(t, err)
	require.Equal(t, "feature", got.Category)
	require.Equal(t, "high", got.Interest)
	require.Equal(t, "adds AI tab", got.Summary)
	require.Equal(t, []string{"prview.go"}, got.KeyFiles)
	require.Equal(t, "@alice approved", got.ReviewStatus)
}

func TestParsePRSummaryStripsMarkdownFences(t *testing.T) {
	raw := "```json\n{\"category\":\"bug\",\"interest\":\"low\",\"summary\":\"fix\",\"key_files\":[],\"review_status\":\"none\"}\n```"
	got, err := ParsePRSummary(raw)
	require.NoError(t, err)
	require.Equal(t, "bug", got.Category)
}

func TestParsePRSummaryMalformedJSON(t *testing.T) {
	_, err := ParsePRSummary("not json at all")
	require.ErrorIs(t, err, ErrParseFailed)
}

// ── ParseNotificationSummary ──────────────────────────────────────────────────

func TestParseNotificationSummaryHappyPath(t *testing.T) {
	raw := `{"interest":"high","summary":"@alice requested your review"}`
	got, err := ParseNotificationSummary(raw)
	require.NoError(t, err)
	require.Equal(t, "high", got.Interest)
	require.Equal(t, "@alice requested your review", got.Summary)
}

func TestParseNotificationSummaryMalformed(t *testing.T) {
	_, err := ParseNotificationSummary("{bad json")
	require.ErrorIs(t, err, ErrParseFailed)
}

// ── ParseMergedPRSummary ──────────────────────────────────────────────────────

func TestParseMergedPRSummaryHappyPath(t *testing.T) {
	raw := `{"what_changed":"added streaming","key_files":["prview.go","aisummary.go"]}`
	got, err := ParseMergedPRSummary(raw)
	require.NoError(t, err)
	require.Equal(t, "added streaming", got.WhatChanged)
	require.Equal(t, []string{"prview.go", "aisummary.go"}, got.KeyFiles)
	require.Nil(t, got.Discussion)
	require.Nil(t, got.RiskNotes)
}

func TestParseMergedPRSummaryMalformed(t *testing.T) {
	_, err := ParseMergedPRSummary("")
	require.ErrorIs(t, err, ErrParseFailed)
}
