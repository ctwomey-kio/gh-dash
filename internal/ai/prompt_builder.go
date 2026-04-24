package ai

import (
	"encoding/json"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
)

type promptFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type promptCommit struct {
	Message string `json:"message"`
}

type promptReview struct {
	Author string `json:"author"`
	State  string `json:"state"`
	Body   string `json:"body,omitempty"`
}

type promptComment struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

type prPromptPayload struct {
	Title              string          `json:"title"`
	Body               string          `json:"body"`
	Author             string          `json:"author"`
	State              string          `json:"state"`
	ReviewDecision     string          `json:"reviewDecision"`
	Additions          int             `json:"additions"`
	Deletions          int             `json:"deletions"`
	IsDraft            bool            `json:"isDraft"`
	Files              []promptFile    `json:"files"`
	Commits            []promptCommit  `json:"commits"`
	Reviews            []promptReview  `json:"reviews"`
	Comments           []promptComment `json:"comments"`
	ViewerReviewState  string          `json:"viewerReviewState,omitempty"`
	ViewerReviewedDate string          `json:"viewerReviewedDate,omitempty"`
}

// BuildPRPromptPayload serializes PR data into the JSON payload for the LLM user message.
// Truncation: body 2000 chars, files 50, commits 50 — matching the Slack prototype limits.
func BuildPRPromptPayload(
	primary *data.PullRequestData,
	enriched data.EnrichedPullRequestData,
) string {
	body := enriched.Body
	if len(body) > 2000 {
		body = body[:2000]
	}

	files := make([]promptFile, 0)
	for i, f := range enriched.Files.Nodes {
		if i >= 50 {
			break
		}
		files = append(files, promptFile{
			Path:      f.Path,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}

	commits := make([]promptCommit, 0)
	for i, node := range enriched.AllCommits.Nodes {
		if i >= 50 {
			break
		}
		commits = append(commits, promptCommit{Message: node.Commit.MessageHeadline})
	}

	reviews := make([]promptReview, 0)
	for _, r := range enriched.Reviews.Nodes {
		b := r.Body
		if len(b) > 300 {
			b = b[:300]
		}
		reviews = append(reviews, promptReview{
			Author: r.Author.Login,
			State:  r.State,
			Body:   b,
		})
	}

	comments := make([]promptComment, 0)
	for _, c := range enriched.Comments.Nodes {
		b := c.Body
		if len(b) > 400 {
			b = b[:400]
		}
		comments = append(comments, promptComment{
			Author: c.Author.Login,
			Body:   b,
		})
	}

	author := primary.Author.Login
	if primary.Author.AsUser.Name != "" {
		author = primary.Author.AsUser.Name
	}

	viewerReviewState := ""
	viewerReviewedDate := ""
	if vlr := enriched.ViewerLatestReview; vlr != nil {
		viewerReviewState = vlr.State
		t := vlr.CreatedAt
		if vlr.SubmittedAt != nil {
			t = *vlr.SubmittedAt
		}
		viewerReviewedDate = t.Format("2006-01-02")
	}

	payload := prPromptPayload{
		Title:              enriched.Title,
		Body:               body,
		Author:             author,
		State:              enriched.State,
		ReviewDecision:     enriched.ReviewDecision,
		Additions:          enriched.Additions,
		Deletions:          enriched.Deletions,
		IsDraft:            enriched.IsDraft,
		Files:              files,
		Commits:            commits,
		Reviews:            reviews,
		Comments:           comments,
		ViewerReviewState:  viewerReviewState,
		ViewerReviewedDate: viewerReviewedDate,
	}

	b, _ := json.Marshal(payload)
	return string(b)
}

type notificationPromptPayload struct {
	Title          string         `json:"title"`
	Body           string         `json:"body,omitempty"`
	Author         string         `json:"author"`
	State          string         `json:"state"`
	ReviewDecision string         `json:"reviewDecision"`
	Additions      int            `json:"additions"`
	Deletions      int            `json:"deletions"`
	IsDraft        bool           `json:"isDraft"`
	TotalFiles     int            `json:"totalFiles"`
	Files          []promptFile   `json:"files,omitempty"`
	Reviews        []promptReview `json:"reviews,omitempty"`
	Reviewers      []string       `json:"reviewers,omitempty"`
	Labels         []string       `json:"labels,omitempty"`
}

// BuildEnrichedNotificationPromptPayload serializes EnrichedPullRequestData into the JSON payload
// for the notification summary LLM call. Used when full PR data is already available (e.g. from
// the notification section's background enrichment fetch).
func BuildEnrichedNotificationPromptPayload(pr data.EnrichedPullRequestData) string {
	body := pr.Body
	if len(body) > 1000 {
		body = body[:1000]
	}

	files := make([]promptFile, 0, len(pr.Files.Nodes))
	for _, f := range pr.Files.Nodes {
		files = append(files, promptFile{
			Path:      f.Path,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}

	reviews := make([]promptReview, 0, len(pr.Reviews.Nodes))
	for _, r := range pr.Reviews.Nodes {
		b := r.Body
		if len(b) > 200 {
			b = b[:200]
		}
		reviews = append(reviews, promptReview{
			Author: r.Author.Login,
			State:  r.State,
			Body:   b,
		})
	}

	reviewers := make([]string, 0, len(pr.ReviewRequests.Nodes))
	for _, req := range pr.ReviewRequests.Nodes {
		if name := req.GetReviewerDisplayName(); name != "" {
			reviewers = append(reviewers, name)
		}
	}

	labels := make([]string, 0, len(pr.Labels.Nodes))
	for _, l := range pr.Labels.Nodes {
		labels = append(labels, l.Name)
	}

	payload := notificationPromptPayload{
		Title:          pr.Title,
		Body:           body,
		Author:         pr.Author.Login,
		State:          pr.State,
		ReviewDecision: pr.ReviewDecision,
		Additions:      pr.Additions,
		Deletions:      pr.Deletions,
		IsDraft:        pr.IsDraft,
		TotalFiles:     pr.Files.TotalCount,
		Files:          files,
		Reviews:        reviews,
		Reviewers:      reviewers,
		Labels:         labels,
	}

	b, _ := json.Marshal(payload)
	return string(b)
}

type addressedPromptPayload struct {
	Title       string         `json:"title"`
	Author      string         `json:"author"`
	CommitCount int            `json:"newCommitCount"`
	Commits     []promptCommit `json:"newCommits"`
	Files       []promptFile   `json:"files,omitempty"`
}

// BuildAddressedPromptPayload serializes the commits pushed after the viewer's review into
// the JSON payload for the AddressedSummary LLM call.
func BuildAddressedPromptPayload(pr data.EnrichedPullRequestData, commitCount int) string {
	commits := make([]promptCommit, 0, commitCount)
	if pr.ViewerLatestReview != nil {
		reviewedAt := pr.ViewerLatestReview.CreatedAt
		if pr.ViewerLatestReview.SubmittedAt != nil {
			reviewedAt = *pr.ViewerLatestReview.SubmittedAt
		}
		for _, node := range pr.AllCommits.Nodes {
			if node.Commit.CommittedDate.After(reviewedAt) {
				commits = append(commits, promptCommit{Message: node.Commit.MessageHeadline})
			}
		}
	}

	files := make([]promptFile, 0, len(pr.Files.Nodes))
	for _, f := range pr.Files.Nodes {
		files = append(files, promptFile{
			Path:      f.Path,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}

	payload := addressedPromptPayload{
		Title:       pr.Title,
		Author:      pr.Author.Login,
		CommitCount: commitCount,
		Commits:     commits,
		Files:       files,
	}

	b, _ := json.Marshal(payload)
	return string(b)
}

// BuildNotificationPromptPayload serializes basic PullRequestData into the JSON payload
// for the notification summary LLM call. Uses only the fields available before enrichment.
func BuildNotificationPromptPayload(pr *data.PullRequestData) string {
	body := pr.Body
	if len(body) > 1000 {
		body = body[:1000]
	}

	files := make([]promptFile, 0, len(pr.Files.Nodes))
	for _, f := range pr.Files.Nodes {
		files = append(files, promptFile{
			Path:      f.Path,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}

	reviews := make([]promptReview, 0, len(pr.Reviews.Nodes))
	for _, r := range pr.Reviews.Nodes {
		b := r.Body
		if len(b) > 200 {
			b = b[:200]
		}
		reviews = append(reviews, promptReview{
			Author: r.Author.Login,
			State:  r.State,
			Body:   b,
		})
	}

	reviewers := make([]string, 0, len(pr.ReviewRequests.Nodes))
	for _, req := range pr.ReviewRequests.Nodes {
		if name := req.GetReviewerDisplayName(); name != "" {
			reviewers = append(reviewers, name)
		}
	}

	labels := make([]string, 0, len(pr.Labels.Nodes))
	for _, l := range pr.Labels.Nodes {
		labels = append(labels, l.Name)
	}

	author := pr.Author.Login
	if pr.Author.AsUser.Name != "" {
		author = pr.Author.AsUser.Name
	}

	payload := notificationPromptPayload{
		Title:          pr.Title,
		Body:           body,
		Author:         author,
		State:          pr.State,
		ReviewDecision: pr.ReviewDecision,
		Additions:      pr.Additions,
		Deletions:      pr.Deletions,
		IsDraft:        pr.IsDraft,
		TotalFiles:     pr.Files.TotalCount,
		Files:          files,
		Reviews:        reviews,
		Reviewers:      reviewers,
		Labels:         labels,
	}

	b, _ := json.Marshal(payload)
	return string(b)
}
