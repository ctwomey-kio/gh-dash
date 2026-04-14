package ai

import "errors"

var (
	ErrNoAPIKey    = errors.New("ANTHROPIC_API_KEY environment variable not set")
	ErrRateLimited = errors.New("Anthropic API rate limited")
	ErrTimeout     = errors.New("AI summary request timed out")
	ErrDisabled    = errors.New("AI features disabled in config")
	ErrParseFailed = errors.New("failed to parse AI summary response")
)
