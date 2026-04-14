// notifytest manually fires a notification for a given PR URL, for testing Phase 2b.
// Usage: go run ./cmd/notifytest <pr-url>
// Example: go run ./cmd/notifytest https://github.com/kontaktio/kio-data-platform/pull/120
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	gh "github.com/cli/go-gh/v2/pkg/api"

	"github.com/dlvhdr/gh-dash/v4/internal/ai"
	pdata "github.com/dlvhdr/gh-dash/v4/internal/data"

	stdctx "context"
)

type ghFile struct {
	Filename  string `json:"filename"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type ghUser struct {
	Login string `json:"login"`
}

type ghTeam struct {
	Slug string `json:"slug"`
}

type ghReview struct {
	User  ghUser `json:"user"`
	State string `json:"state"`
	Body  string `json:"body"`
}

type ghReviewers struct {
	Users []ghUser `json:"users"`
	Teams []ghTeam `json:"teams"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghPR struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	State          string    `json:"state"`
	Draft          bool      `json:"draft"`
	Additions      int       `json:"additions"`
	Deletions      int       `json:"deletions"`
	ChangedFiles   int       `json:"changed_files"`
	UpdatedAt      time.Time `json:"updated_at"`
	ReviewDecision string    `json:"auto_merge"` // placeholder
	Labels         []ghLabel `json:"labels"`
	User           ghUser    `json:"user"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: notifytest <pr-url>")
		os.Exit(1)
	}

	prURL := os.Args[1]
	// parse owner/repo/number from URL
	// https://github.com/owner/repo/pull/NNN
	parts := strings.Split(strings.TrimPrefix(prURL, "https://github.com/"), "/")
	if len(parts) < 4 {
		fmt.Fprintln(os.Stderr, "invalid PR URL:", prURL)
		os.Exit(1)
	}
	owner, repo, num := parts[0], parts[1], parts[3]
	apiPath := fmt.Sprintf("repos/%s/%s/pulls/%s", owner, repo, num)

	client, err := gh.DefaultRESTClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gh REST client:", err)
		os.Exit(1)
	}

	var pr ghPR
	if err := client.Get(apiPath, &pr); err != nil {
		fmt.Fprintln(os.Stderr, "fetch PR:", err)
		os.Exit(1)
	}

	var files []ghFile
	if err := client.Get(apiPath+"/files", &files); err != nil {
		fmt.Fprintln(os.Stderr, "fetch files:", err)
	}

	var reviews []ghReview
	if err := client.Get(apiPath+"/reviews", &reviews); err != nil {
		fmt.Fprintln(os.Stderr, "fetch reviews:", err)
	}

	var reqReviewers ghReviewers
	if err := client.Get(apiPath+"/requested_reviewers", &reqReviewers); err != nil {
		fmt.Fprintln(os.Stderr, "fetch reviewers:", err)
	}

	// Build pdata.PullRequestData from REST responses
	prd := &pdata.PullRequestData{
		Number:    pr.Number,
		Title:     pr.Title,
		Body:      pr.Body,
		State:     strings.ToUpper(pr.State),
		IsDraft:   pr.Draft,
		Additions: pr.Additions,
		Deletions: pr.Deletions,
		Url:       prURL,
		UpdatedAt: pr.UpdatedAt,
	}
	prd.Author.Login = pr.User.Login
	prd.Repository.NameWithOwner = owner + "/" + repo

	// Review decision — not in REST easily; use "REVIEW_REQUIRED" as default
	prd.ReviewDecision = "REVIEW_REQUIRED"

	for i, f := range files {
		if i >= 5 {
			break
		}
		prd.Files.Nodes = append(prd.Files.Nodes, pdata.ChangedFile{
			Path:      f.Filename,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}
	prd.Files.TotalCount = pr.ChangedFiles

	for i, r := range reviews {
		if i >= 3 {
			break
		}
		body := r.Body
		if len(body) > 200 {
			body = body[:200]
		}
		prd.Reviews.Nodes = append(prd.Reviews.Nodes, pdata.Review{
			Author: struct{ Login string }{Login: r.User.Login},
			State:  r.State,
			Body:   body,
		})
	}

	for i, u := range reqReviewers.Users {
		if i >= 5 {
			break
		}
		prd.ReviewRequests.Nodes = append(prd.ReviewRequests.Nodes, pdata.ReviewRequestNode{})
		_ = u // ReviewRequestNode uses a union type — just print names separately
	}
	for i, t := range reqReviewers.Teams {
		if i >= 5 {
			break
		}
		_ = t
		_ = i
	}

	for i, l := range pr.Labels {
		if i >= 6 {
			break
		}
		prd.Labels.Nodes = append(prd.Labels.Nodes, pdata.Label{Name: l.Name})
	}

	// Build payload
	payload := ai.BuildNotificationPromptPayload(prd)
	fmt.Println("=== Payload ===")
	fmt.Println(payload)
	fmt.Println()

	// Call AI
	aiClient, err := ai.NewClient("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI client:", err)
		os.Exit(1)
	}

	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 10*time.Second)
	defer cancel()

	raw, err := aiClient.GenerateSummary(ctx, ai.Request{
		Mode:    ai.NotificationSummary,
		Payload: payload,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "AI call:", err)
		os.Exit(1)
	}

	fmt.Println("=== Raw AI response ===")
	fmt.Println(raw)
	fmt.Println()

	resp, err := ai.ParseNotificationSummary(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}

	author := pr.User.Login
	badge := fmt.Sprintf("[%s]", resp.Interest)
	title := fmt.Sprintf("gh-dash %s · %s", badge, author)
	subtitle := fmt.Sprintf("PR #%d · +%d −%d · %d files", pr.Number, pr.Additions, pr.Deletions, pr.ChangedFiles)
	message := pr.Title + "\n" + resp.Summary

	fmt.Println("=== Notification ===")
	fmt.Println("Title:   ", title)
	fmt.Println("Subtitle:", subtitle)
	fmt.Println("Message: ", message)
	fmt.Println()

	// Fire terminal-notifier
	if tn, err := exec.LookPath("terminal-notifier"); err == nil {
		args := []string{
			"-title", title,
			"-subtitle", subtitle,
			"-message", message,
			"-group", "gh-dash-test",
			"-open", prURL,
		}
		if err := exec.Command(tn, args...).Run(); err != nil {
			fmt.Fprintln(os.Stderr, "terminal-notifier:", err)
		} else {
			fmt.Println("Notification sent via terminal-notifier.")
		}
	} else {
		fmt.Println("(terminal-notifier not found, skipping notification)")
	}
}
