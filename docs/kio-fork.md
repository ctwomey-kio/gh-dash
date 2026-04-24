# gh-dash — Kontakt.io fork

This is the Kontakt.io fork of [dlvhdr/gh-dash](https://github.com/dlvhdr/gh-dash), a terminal UI for GitHub pull requests and issues. The fork lives on the `kio-fork` branch and `main` mirrors upstream, so upstream updates are a clean rebase away.

## What this fork adds

- **AI PR summaries** — an "AI Summary" tab appears first in the PR sidebar. It streams a structured summary (category, interest level, key files, review status, risk notes) from Claude (haiku-4-5 by default). Merged PRs get a changelog-style summary instead. The summary is viewer-aware: if you have already approved or requested changes, the AI leads with that context.
- **Desktop notifications** — desktop alerts fire from the GitHub notification queue. Alerts are AI-enriched with a one-line summary and diff stats. The "addressed your comments" signal detects when new commits are pushed after your last review and surfaces a separate notification. Requires `terminal-notifier` (preferred) or falls back to `osascript`.
- **Requested-teams column** — shows which teams have been asked to review a PR, alongside the existing review-status column.
- **Toggle merged/open (`M`)** — press `M` in any PR section to switch between open and merged PR views.

## Prerequisites

| Tool | Purpose | Install |
|------|---------|---------|
| Go 1.24+ | build from source | `brew install go` |
| `gh` CLI | authenticated GitHub access | `brew install gh` then `gh auth login` |
| `terminal-notifier` | desktop notifications (macOS) | `brew install terminal-notifier` |
| `ANTHROPIC_API_KEY` | AI summaries | set in shell profile (ask #devops for the team key) |

You do **not** need devbox to run the binary; it is only needed to reproduce the exact upstream CI toolchain (gofumpt, golangci-lint).

## Install

### Option A — `gh extension` (recommended for daily use)

```bash
gh extension install ctwomey-kio/gh-dash
```

This installs the binary as `gh dash`. To update:

```bash
gh extension upgrade dash
```

### Option B — clone and build (recommended for development)

```bash
git clone https://github.com/ctwomey-kio/gh-dash.git
cd gh-dash
go install .
```

The binary lands in `$(go env GOPATH)/bin/gh-dash`.

## Config

Place your config at `~/.config/gh-dash/config.yml`. A minimal config that enables AI and notifications:

```yaml
# yaml-language-server: $schema=https://gh-dash.dev/schema.json

ai:
  enabled: true
  model: claude-haiku-4-5-20251001   # optional — this is the default

prSections:
  - title: Needs my review
    filters: is:open review-requested:@me sort:updated-desc

notificationsSections:
  - title: Inbox
    filters: is:unread
    notify: true   # enables desktop notifications for this section
```

The full config schema lives in the [upstream docs](https://dlvhdr.github.io/gh-dash/configuration/).

## Running

```bash
# via gh extension
gh dash

# via clone
gh-dash
```

Logs are written to `./debug.log` when you launch with the `DEBUG=1` environment variable set.

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
| AI tab stays empty | Check `ANTHROPIC_API_KEY` is exported and `ai.enabled: true` in config |
| No desktop notifications | Run `which terminal-notifier`; if missing, `brew install terminal-notifier`. Also verify `notify: true` is set on the notification section in config |
| Notifications fire on every launch | The dedup store lives at `~/.local/share/gh-dash/notified.json`; delete it to reset |
| Build fails with `go: module not found` | Run `go mod tidy` — the Anthropic SDK may need a fresh download |

## Why this fork exists

The Kontakt.io engineering team uses gh-dash for daily PR triage. We added AI summaries and desktop notifications to reduce context-switching: instead of opening every PR to understand its state, the terminal surface shows enough to decide what to review next.

These features are in progress of being cleaned up for potential upstream submission (see AI_POLICY.md for the disclosure requirements). In the meantime, this fork ships what we use.
