package ai

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Request is the input to GenerateSummary / StreamSummary.
type Request struct {
	Mode    PromptMode
	Payload string // JSON-serialized PR context for the user message
}

// RulesConfig holds per-mode paths to Markdown files that override the default
// behavioural rules (interest levels, review_status framing) in each system prompt.
type RulesConfig struct {
	PRSummary    string // sidebar PR summary (open PRs)
	Notification string // desktop notification one-liner
	Addressed    string // "new commits since your review" summary
}

// Client wraps the Anthropic SDK and is safe for concurrent use.
type Client struct {
	inner *anthropic.Client
	model string
	rules map[PromptMode]string // loaded rule overrides keyed by mode; absent = use defaults
}

// NewClient creates a Client. Returns an error if ANTHROPIC_API_KEY is not set.
// Each non-empty path in rules is read at init time; missing or unreadable files
// fall back silently to the compiled-in defaults.
func NewClient(model string, rules RulesConfig) (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, ErrNoAPIKey
	}
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	loaded := make(map[PromptMode]string)
	for mode, path := range map[PromptMode]string{
		PRSummary:           rules.PRSummary,
		NotificationSummary: rules.Notification,
		AddressedSummary:    rules.Addressed,
	} {
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				path = filepath.Join(home, path[2:])
			}
		}
		if b, err := os.ReadFile(path); err == nil {
			if content := strings.TrimSpace(string(b)); content != "" {
				loaded[mode] = content
			}
		}
	}

	c := anthropic.NewClient(option.WithAPIKey(key))
	return &Client{inner: &c, model: model, rules: loaded}, nil
}

// GenerateSummary sends a request to the Anthropic API and returns the raw text response.
func (c *Client) GenerateSummary(ctx context.Context, req Request) (string, error) {
	msg, err := c.inner.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{
				Text:         SystemPrompt(req.Mode, c.rules[req.Mode]),
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Payload)),
		},
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", ErrTimeout
		}
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			if apiErr.StatusCode == http.StatusTooManyRequests {
				return "", ErrRateLimited
			}
		}
		return "", err
	}

	if len(msg.Content) == 0 {
		return "", ErrParseFailed
	}
	return msg.Content[0].Text, nil
}

// StreamSummary streams the response, sending text chunks to tokenCh as they arrive.
// It returns the full accumulated text and any error. tokenCh is NOT closed by this method.
// The caller should close tokenCh after this returns.
func (c *Client) StreamSummary(
	ctx context.Context,
	req Request,
	tokenCh chan<- string,
) (string, error) {
	var accum strings.Builder
	stream := c.inner.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{
				Text:         SystemPrompt(req.Mode, c.rules[req.Mode]),
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Payload)),
		},
	})

	for stream.Next() {
		event := stream.Current()
		if event.Type == "content_block_delta" {
			if text := event.Delta.Text; text != "" {
				accum.WriteString(text)
				select {
				case tokenCh <- text:
				case <-ctx.Done():
					return accum.String(), ctx.Err()
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return accum.String(), ErrTimeout
		}
		if errors.Is(err, context.Canceled) {
			return accum.String(), err
		}
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
			return accum.String(), ErrRateLimited
		}
		return accum.String(), err
	}

	return accum.String(), nil
}
