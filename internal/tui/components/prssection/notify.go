package prssection

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/ai"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	pctx "github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

// notifContent holds the visible slots of a macOS notification.
// badge goes into the title so it's always visible in the collapsed banner.
// subtitle is shown in Alerts style (not guaranteed in Banners).
// message is the full body, visible on expand.
type notifContent struct {
	badge    string // e.g. "[MED]" — merged into title for banner visibility
	subtitle string // PR title + number line
	message  string // AI summary or stats line
}

func notifyNewPR(ctx *pctx.ProgramContext, pr *data.PullRequestData) {
	author := pr.GetAuthorDisplayName()

	var content notifContent
	if ai := buildAINotifContent(ctx, pr); ai != nil {
		content = *ai
	} else {
		content = buildStatsContent(pr)
	}

	// Interest badge goes in the title so it's visible in the unexpanded banner.
	// subtitle is shown beneath title in Alerts style; message is shown on expand.
	var title string
	if content.badge != "" {
		title = fmt.Sprintf("gh-dash %s · %s", content.badge, author)
	} else {
		title = fmt.Sprintf("gh-dash · %s", author)
	}

	url := pr.Url
	go func() {
		// Prefer terminal-notifier (supports -subtitle, click-to-open)
		if tn, err := exec.LookPath("terminal-notifier"); err == nil {
			args := []string{
				"-title", title,
				"-message", content.message,
				"-group", "gh-dash",
			}
			if content.subtitle != "" {
				args = append(args, "-subtitle", content.subtitle)
			}
			if url != "" {
				args = append(args, "-open", url)
			}
			if err := exec.Command(tn, args...).Run(); err != nil {
				log.Error("terminal-notifier failed", "err", err)
			}
			return
		}
		// Fall back to osascript (no subtitle support)
		body := content.subtitle + "\n" + content.message
		script := fmt.Sprintf("display notification %q with title %q", body, title)
		if err := exec.Command("osascript", "-e", script).Run(); err != nil {
			log.Error("osascript notification failed", "err", err)
		}
	}()
}

// buildAINotifContent attempts to produce an AI-generated notification.
// Returns nil on any failure so the caller can fall back to stats format.
func buildAINotifContent(ctx *pctx.ProgramContext, pr *data.PullRequestData) *notifContent {
	if ctx == nil || ctx.AIClient == nil || ctx.AINotifCache == nil {
		return nil
	}

	key := ai.CacheKey{URL: pr.Url, UpdatedAt: pr.UpdatedAt}

	// Check notification-specific cache
	if cached, ok := ctx.AINotifCache.Get(key); ok {
		return notifFromResponse(pr, cached)
	}

	// Check sidebar cache — if the user already viewed this PR, reuse the full summary
	if ctx.AICache != nil {
		if full, ok := ctx.AICache.Get(key); ok {
			return notifFromFull(pr, full)
		}
	}

	// Fire a synchronous AI call with a 5s timeout
	callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := ai.BuildNotificationPromptPayload(pr)
	raw, err := ctx.AIClient.GenerateSummary(callCtx, ai.Request{
		Mode:    ai.NotificationSummary,
		Payload: payload,
	})
	if err != nil {
		log.Debug("AI notification summary failed", "err", err, "pr", pr.Number)
		return nil
	}

	resp, err := ai.ParseNotificationSummary(raw)
	if err != nil {
		log.Debug("AI notification summary parse failed", "err", err, "pr", pr.Number)
		return nil
	}

	ctx.AINotifCache.Set(key, resp)
	return notifFromResponse(pr, resp)
}

func notifFromResponse(pr *data.PullRequestData, resp ai.NotificationSummaryResponse) *notifContent {
	return &notifContent{
		badge:    fmt.Sprintf("[%s]", resp.Interest),
		subtitle: statsLine(pr),
		message:  pr.Title + "\n" + resp.Summary,
	}
}

func notifFromFull(pr *data.PullRequestData, full ai.PRSummaryResponse) *notifContent {
	return &notifContent{
		badge:    fmt.Sprintf("[%s]", full.Interest),
		subtitle: statsLine(pr),
		message:  pr.Title + "\n" + truncateToSentences(full.Summary, 150),
	}
}

// statsLine returns a compact diff+file count line that always fits in the subtitle.
// Uses Unicode minus (−) to visually distinguish deletions from additions.
func statsLine(pr *data.PullRequestData) string {
	return fmt.Sprintf("PR #%d · +%d −%d · %d files", pr.Number, pr.Additions, pr.Deletions, pr.Files.TotalCount)
}

// truncateToSentences truncates s to at most maxLen characters,
// preferring to cut at a sentence boundary (". ") if one exists before maxLen.
func truncateToSentences(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	cut := s[:maxLen]
	if idx := strings.LastIndex(cut, ". "); idx > 0 {
		return s[:idx+1]
	}
	return cut + "…"
}

func buildStatsContent(pr *data.PullRequestData) notifContent {
	return notifContent{
		badge:    "",
		subtitle: statsLine(pr),
		message:  pr.Title + "\n" + pr.Repository.NameWithOwner + " · " + reviewDecisionLabel(pr.ReviewDecision),
	}
}

func reviewDecisionLabel(decision string) string {
	switch decision {
	case "APPROVED":
		return "Approved"
	case "CHANGES_REQUESTED":
		return "Changes requested"
	case "REVIEW_REQUIRED":
		return "Review required"
	default:
		return "Awaiting review"
	}
}
