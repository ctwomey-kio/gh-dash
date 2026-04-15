package data

import (
	"testing"
	"time"

	gh "github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/require"
)

func ptrTime(t time.Time) *time.Time { return &t }

func TestClearEnrichmentCache(t *testing.T) {
	// Save original state
	originalCachedClient := cachedClient
	defer func() {
		cachedClient = originalCachedClient
	}()

	t.Run("clears nil cache without panic", func(t *testing.T) {
		cachedClient = nil
		require.True(t, IsEnrichmentCacheCleared(), "cache should be cleared initially")

		ClearEnrichmentCache()
		require.True(t, IsEnrichmentCacheCleared(), "cache should remain cleared")
	})

	t.Run("clears non-nil cache", func(t *testing.T) {
		// Simulate having a cached client (we use an empty struct pointer
		// since we can't create a real GraphQL client without credentials)
		cachedClient = &gh.GraphQLClient{}
		require.False(
			t,
			IsEnrichmentCacheCleared(),
			"cache should not be cleared when client is set",
		)

		ClearEnrichmentCache()
		require.True(
			t,
			IsEnrichmentCacheCleared(),
			"cache should be cleared after ClearEnrichmentCache",
		)
	})
}

func TestIsEnrichmentCacheCleared(t *testing.T) {
	// Save original state
	originalCachedClient := cachedClient
	defer func() {
		cachedClient = originalCachedClient
	}()

	t.Run("returns true when cache is nil", func(t *testing.T) {
		cachedClient = nil
		require.True(t, IsEnrichmentCacheCleared())
	})

	t.Run("returns false when cache is set", func(t *testing.T) {
		cachedClient = &gh.GraphQLClient{}
		require.False(t, IsEnrichmentCacheCleared())
	})
}

func makeCommitNode(committedDate time.Time, msg string) struct {
	Commit struct {
		AbbreviatedOid  string
		CommittedDate   time.Time
		MessageHeadline string
		Author          struct {
			Name string
			User struct {
				Login string
			}
		}
		StatusCheckRollup StatusCheckRollupStats
	}
} {
	var n struct {
		Commit struct {
			AbbreviatedOid  string
			CommittedDate   time.Time
			MessageHeadline string
			Author          struct {
				Name string
				User struct {
					Login string
				}
			}
			StatusCheckRollup StatusCheckRollupStats
		}
	}
	n.Commit.CommittedDate = committedDate
	n.Commit.MessageHeadline = msg
	return n
}

func TestCommitsSinceReview(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)
	threeHoursAgo := now.Add(-3 * time.Hour)

	t.Run("nil viewerLatestReview returns zero", func(t *testing.T) {
		pr := EnrichedPullRequestData{}
		count, reviewedAt := CommitsSinceReview(pr)
		require.Equal(t, 0, count)
		require.True(t, reviewedAt.IsZero())
	})

	t.Run("no commits newer than review returns zero", func(t *testing.T) {
		pr := EnrichedPullRequestData{
			ViewerLatestReview: &ViewerLatestReview{
				State:       "APPROVED",
				SubmittedAt: ptrTime(oneHourAgo),
				CreatedAt:   twoHoursAgo,
			},
			AllCommits: AllCommits{Nodes: []struct {
				Commit struct {
					AbbreviatedOid  string
					CommittedDate   time.Time
					MessageHeadline string
					Author          struct {
						Name string
						User struct {
							Login string
						}
					}
					StatusCheckRollup StatusCheckRollupStats
				}
			}{
				makeCommitNode(twoHoursAgo, "old commit"),
				makeCommitNode(threeHoursAgo, "older commit"),
			}},
		}
		count, _ := CommitsSinceReview(pr)
		require.Equal(t, 0, count)
	})

	t.Run("counts commits after submittedAt", func(t *testing.T) {
		pr := EnrichedPullRequestData{
			ViewerLatestReview: &ViewerLatestReview{
				State:       "CHANGES_REQUESTED",
				SubmittedAt: ptrTime(twoHoursAgo),
				CreatedAt:   threeHoursAgo,
			},
			AllCommits: AllCommits{Nodes: []struct {
				Commit struct {
					AbbreviatedOid  string
					CommittedDate   time.Time
					MessageHeadline string
					Author          struct {
						Name string
						User struct {
							Login string
						}
					}
					StatusCheckRollup StatusCheckRollupStats
				}
			}{
				makeCommitNode(now, "fix: address comments"),
				makeCommitNode(oneHourAgo, "fix: another change"),
				makeCommitNode(threeHoursAgo, "old commit before review"),
			}},
		}
		count, reviewedAt := CommitsSinceReview(pr)
		require.Equal(t, 2, count)
		require.True(t, reviewedAt.Equal(twoHoursAgo))
	})

	t.Run("falls back to createdAt when submittedAt is nil", func(t *testing.T) {
		pr := EnrichedPullRequestData{
			ViewerLatestReview: &ViewerLatestReview{
				State:       "PENDING",
				SubmittedAt: nil,
				CreatedAt:   twoHoursAgo,
			},
			AllCommits: AllCommits{Nodes: []struct {
				Commit struct {
					AbbreviatedOid  string
					CommittedDate   time.Time
					MessageHeadline string
					Author          struct {
						Name string
						User struct {
							Login string
						}
					}
					StatusCheckRollup StatusCheckRollupStats
				}
			}{
				makeCommitNode(now, "commit after pending review"),
				makeCommitNode(threeHoursAgo, "commit before review"),
			}},
		}
		count, reviewedAt := CommitsSinceReview(pr)
		require.Equal(t, 1, count)
		require.True(t, reviewedAt.Equal(twoHoursAgo))
	})

	t.Run("prefers submittedAt over createdAt", func(t *testing.T) {
		// SubmittedAt is later than CreatedAt; commit between them should NOT be counted
		justBeforeSubmit := oneHourAgo.Add(-1 * time.Minute)
		pr := EnrichedPullRequestData{
			ViewerLatestReview: &ViewerLatestReview{
				State:       "APPROVED",
				SubmittedAt: ptrTime(oneHourAgo),
				CreatedAt:   twoHoursAgo,
			},
			AllCommits: AllCommits{Nodes: []struct {
				Commit struct {
					AbbreviatedOid  string
					CommittedDate   time.Time
					MessageHeadline string
					Author          struct {
						Name string
						User struct {
							Login string
						}
					}
					StatusCheckRollup StatusCheckRollupStats
				}
			}{
				makeCommitNode(now, "after submittedAt — should count"),
				makeCommitNode(justBeforeSubmit, "just before submittedAt — should NOT count"),
			}},
		}
		count, _ := CommitsSinceReview(pr)
		require.Equal(t, 1, count)
	})
}

func TestSetClient(t *testing.T) {
	// Save original state
	originalClient := client
	originalCachedClient := cachedClient
	defer func() {
		client = originalClient
		cachedClient = originalCachedClient
	}()

	t.Run("sets both client and cachedClient", func(t *testing.T) {
		client = nil
		cachedClient = nil

		// SetClient with nil should set both to nil
		SetClient(nil)
		require.Nil(t, client)
		require.True(t, IsEnrichmentCacheCleared())
	})
}
