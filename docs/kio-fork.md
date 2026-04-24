# gh-dash fork — AI summaries + desktop notifications

A personal fork of [dlvhdr/gh-dash](https://github.com/dlvhdr/gh-dash), a terminal UI for GitHub pull requests and issues. The fork lives on the `kio-fork` branch; `main` mirrors upstream so it stays rebaseable.

## What this fork adds

- **AI PR summaries** — an "AI Summary" tab appears first in the PR sidebar. It streams a structured summary (category, interest level, key files, review status, risk notes) from Claude. Merged PRs get a changelog-style summary instead. The summary is viewer-aware: if you've already approved or requested changes, the AI leads with that context.
- **Desktop notifications** — alerts fire from the GitHub notification queue for direct review requests and team mentions. Notifications are AI-enriched with a one-line summary and diff stats. An "addressed your comments" signal fires when new commits arrive after your last review.
- **Requested-teams column** — shows which teams have been asked to review a PR, alongside the existing review-status column. Your own teams are bold; others are faint.
- **Toggle merged/open (`M`)** — press `M` in any PR section to switch between open and merged PR views.

## Prerequisites

| Tool | Purpose | Install |
|------|---------|---------|
| `gh` CLI | install and run the extension | `brew install gh` then `gh auth login` |
| `terminal-notifier` | desktop notifications (macOS) | `brew install terminal-notifier` |
| `ANTHROPIC_API_KEY` | AI summaries | export in your shell profile |
| Go 1.24+ | only needed for Option B (clone and build) | `brew install go` |

## Install

### Option A — `gh extension` (recommended for daily use)

```bash
gh extension install ctwomey-kio/gh-dash
```

This installs the binary as `gh dash`. To update:

```bash
gh extension upgrade dash
```

### Option B — clone and build

```bash
git clone https://github.com/ctwomey-kio/gh-dash.git
cd gh-dash
go install .
```

The binary lands in `$(go env GOPATH)/bin/gh-dash`.

## Config

Place your config at `~/.config/gh-dash/config.yml`. A starter config that enables all fork features is at `docs/config-template.yml` in this repo. The key additions vs. a vanilla gh-dash config:

```yaml
# Enable AI summaries (requires ANTHROPIC_API_KEY)
ai:
  enabled: true
  model: claude-haiku-4-5-20251001   # optional — this is the default

prSections:
  - title: "Direct Review"
    filters: "is:open user-review-requested:@me sort:updated-desc"
  - title: "Team Review"
    filters: "is:open review-requested:@me -user-review-requested:@me sort:updated-desc"
    layout:
      requestedTeams:   # shows the requested-teams column for this section
        hidden: false
        width: 12

# Desktop notifications fire from notificationsSections with notify: true.
# Only review_requested and team_mention reasons trigger alerts.
notificationsSections:
  - title: "Review Requested"
    filters: "reason:review-requested"
    notify: true
  - title: "Participating"
    filters: "reason:participating"

# Keeps already-reviewed PRs visible — useful with the AI summary tab
smartFilteringAtLaunch: false
showAuthorIcons: true
```

The full config schema lives in the [upstream docs](https://dlvhdr.github.io/gh-dash/configuration/).

## Running

```bash
# via gh extension
gh dash

# via clone
gh-dash
```

## AI PR summaries

When you open a PR the sidebar loads an **AI Summary** tab first. For open PRs it shows interest level, category, key files, review status, and any risk notes. For merged PRs it switches to a changelog framing (what changed, discussion highlights).

**Config knobs:**

```yaml
ai:
  enabled: true
  model: claude-haiku-4-5-20251001
```

Any model ID from the [Anthropic docs](https://docs.anthropic.com/en/docs/about-claude/models) works. Haiku is the default for speed and cost.

**Cache:** Summaries are cached in-memory for the process lifetime, keyed by `{PR URL, UpdatedAt, your review state}`. The summary re-fetches automatically when the PR gets new activity or your review state changes. To force a refresh, restart `gh dash`.

**Prompts:** The system prompts and payload builders live in `internal/ai/prompt.go` and `internal/ai/prompt_builder.go`. They're compiled into the binary — to change the wording or structure, edit those files and rebuild (`go install .`).

**Failure:** If the stream errors mid-way you'll see partial output with an error line. There's no automatic retry — press `[` then `]` to re-render the tab or restart.

## Desktop notifications

Notifications fire while `gh dash` is running. To receive them from login, configure a terminal profile that runs `gh dash` as its startup command and set that profile to launch on login.

**Requirements:** macOS only. `terminal-notifier` is preferred (supports subtitle and click-to-open); falls back to `osascript`. There is no notification backend for Linux today.

**What triggers an alert:** Only `review_requested` (direct review ask) and `team_mention` (CODEOWNERS / team review request) reasons fire desktop alerts — state changes, comments, and other activity are intentionally excluded. The section filters you set narrow which notifications are fetched; the reason gating applies on top.

> **Migrating from an older config?** `notify: true` on `prSections` is a no-op — it was removed when the notification system was rewritten. Move `notify: true` to your `notificationsSections` entries.

**"Addressed your comments" signal:** When new commits arrive on a PR after your last review (approved or changes-requested), a separate notification fires with a "N new commits since your review" subtitle. This fires even if the PR itself wouldn't otherwise match your reason filter.

**First launch:** On first run, all current notifications are marked as seen without firing — this prevents an alert flood when you first set things up.

**Dedup store:** `~/.local/state/gh-dash/notified.json`. Entries are keyed by notification ID and pruned after 90 days. Delete this file to reset dedup state (next launch will re-seed without firing).

**Verify it's working:**

```bash
terminal-notifier -title "gh-dash test" -message "notifications are working" -group gh-dash
```

If you see the notification, the backend is functional. If not, check `System Settings > Notifications > terminal-notifier` and ensure alerts are allowed.

## Keybindings added by this fork

| Key | Action |
|---|---|
| `M` | Toggle current section between open and merged PRs |

The sidebar tab order also changed: **AI Summary** is now first (tab 0). Use `[` / `]` to navigate tabs as before.

## Subtle UI changes

A few things that differ from upstream and may surprise you:

- **Reviewed PRs sink to the bottom** of each section and render faint. PRs you've approved or requested changes on are deprioritised so fresh reviews stay at the top.
- **Author names** show as `"Jane S"` when the GitHub profile has a full name, rather than `@login`. Applies in PR rows, the PR view header, AI prompts, and notification titles.
- **Requested-teams column ("Via")** is hidden by default globally; enable per-section via `layout.requestedTeams: { hidden: false }` as shown in the config snippet above.

## Environment variables

| Variable | Effect |
|---|---|
| `ANTHROPIC_API_KEY` | Required for AI summaries and AI-enriched notification titles. Export in your shell profile. |
| `DEBUG=1` | Writes verbose logs to `./debug.log` in the working directory. Useful for diagnosing notification firing and AI stream issues. |

## Updating

**Extension install:**

```bash
gh extension upgrade dash
```

**Clone — track upstream:**

```bash
git fetch upstream
git rebase upstream/main
```

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| AI tab stays empty | Check `ANTHROPIC_API_KEY` is exported and `ai.enabled: true` is in config |
| AI summary shows partial text | Stream was interrupted. Press `[` then `]` to re-render, or restart `gh dash`. No automatic retry. |
| AI summary doesn't refresh | Cache is keyed by `UpdatedAt`. If GitHub hasn't stamped new activity the cached summary is used. Restart to force a fresh fetch. |
| No desktop notifications | Run `which terminal-notifier`; if missing, `brew install terminal-notifier`. Verify `notify: true` is set on a `notificationsSections` entry (not `prSections`). Also check System Settings > Notifications > terminal-notifier. |
| Getting no alerts for comments or merges | Expected — only `review_requested` and `team_mention` reasons trigger alerts. Other activity is intentionally excluded. |
| Notifications fire on every launch | The dedup store lives at `~/.local/state/gh-dash/notified.json`; delete it to reset. |
| No notifications on Linux | The notification backend is macOS-only. Not supported on Linux today. |
| Build fails with `go: module not found` | (clone only) Run `go mod tidy` — the Anthropic SDK may need a fresh download |
