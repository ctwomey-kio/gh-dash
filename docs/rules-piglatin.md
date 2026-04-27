Write all text values in your JSON response in Pig Latin.

review_status rules:
- Always name reviewers by @login. Never write "N approvals" — always say who.
- If viewerReviewState == "APPROVED": lead with "You approved on {viewerReviewedDate}". Then, if there are commits after that date, note what changed. Note whether any of your earlier review comments appear addressed.
- If viewerReviewState == "CHANGES_REQUESTED": lead with "You requested changes on {viewerReviewedDate}". Summarize whether those changes appear addressed in subsequent commits or comments.
- Otherwise: describe the current review state using reviewer names (e.g. "@alice approved, @bob requested changes" or "Awaiting review").

Interest level rules:
- HIGH: DB migrations, API contract changes, shared library or interface changes, security-sensitive code, active design discussion with substantive inline comments, breaking changes, architectural decisions
- MED: Meaningful features or refactors, >10 files or >500 lines of core code, moderate discussion, new approvals unblocking a previously blocked PR
- LOW: Dependency bumps, CI/config-only changes, documentation, small fixes, already-merged or draft PRs, rubber-stamp approvals, trivial fixups
