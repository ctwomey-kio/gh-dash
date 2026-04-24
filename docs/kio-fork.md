# gh-dash fork — AI summaries + desktop notifications

A personal fork of [dlvhdr/gh-dash](https://github.com/dlvhdr/gh-dash), a terminal UI for GitHub pull requests and issues. The fork lives on the `kio-fork` branch; `main` mirrors upstream so it stays rebaseable.

## What this fork adds

- **AI PR summaries** — an "AI Summary" tab appears first in the PR sidebar. It streams a structured summary (category, interest level, key files, review status, risk notes) from Claude (haiku-4-5 by default). Merged PRs get a changelog-style summary instead. The summary is viewer-aware: if you've already approved or requested changes, the AI leads with that context.
- **Desktop notifications** — alerts fire from the GitHub notification queue. Notifications are AI-enriched with a one-line summary and diff stats. An "addressed your comments" signal detects when new commits arrive after your last review. Requires `terminal-notifier` on macOS (preferred) or falls back to `osascript`.
- **Requested-teams column** — shows which teams have been asked to review a PR, alongside the existing review-status column.
- **Toggle merged/open (`M`)** — press `M` in any PR section to switch between open and merged PR views.

## Prerequisites

| Tool | Purpose | Install |
|------|---------|---------|
| Go 1.24+ | build from source | `brew install go` |
| `gh` CLI | authenticated GitHub access | `brew install gh` then `gh auth login` |
| `terminal-notifier` | desktop notifications (macOS) | `brew install terminal-notifier` |
| `ANTHROPIC_API_KEY` | AI summaries | get one at console.anthropic.com and export in your shell profile |

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

Logs are written to `./debug.log` when you launch with `DEBUG=1` set in your environment.

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
| No desktop notifications | Run `which terminal-notifier`; if missing, `brew install terminal-notifier`. Also verify `notify: true` is set on the notification section |
| Notifications fire on every launch | The dedup store lives at `~/.local/share/gh-dash/notified.json`; delete it to reset |
| Build fails with `go: module not found` | Run `go mod tidy` — the Anthropic SDK may need a fresh download |
