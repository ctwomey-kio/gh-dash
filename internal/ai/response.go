package ai

import (
	"encoding/json"
	"strings"
)

// PRSummaryResponse is the structured output from the LLM for a single PR.
type PRSummaryResponse struct {
	Category     string   `json:"category"`
	Interest     string   `json:"interest"`
	Summary      string   `json:"summary"`
	KeyFiles     []string `json:"key_files"`
	ReviewStatus string   `json:"review_status"`
	RiskNotes    *string  `json:"risk_notes"`
}

// NotificationSummaryResponse is the structured output from the LLM for a notification body.
type NotificationSummaryResponse struct {
	Interest string `json:"interest"`
	Summary  string `json:"summary"`
}

// cleanLLMJSON strips markdown code fences and extracts the JSON object from raw LLM output.
func cleanLLMJSON(raw string) string {
	s := strings.TrimSpace(raw)

	// Strip markdown code fences (```json ... ``` or ``` ... ```)
	if strings.HasPrefix(s, "```") {
		end := strings.LastIndex(s, "```")
		if end > 3 {
			s = s[3:end]
			// Strip optional language tag (e.g. "json\n")
			if nl := strings.Index(s, "\n"); nl != -1 {
				s = s[nl+1:]
			}
			s = strings.TrimSpace(s)
		}
	}

	// Find the first { and last } to extract the JSON object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		s = s[start : end+1]
	}

	return s
}

// ParseNotificationSummary unmarshals the LLM's text response into NotificationSummaryResponse.
func ParseNotificationSummary(raw string) (NotificationSummaryResponse, error) {
	s := cleanLLMJSON(raw)
	var result NotificationSummaryResponse
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return NotificationSummaryResponse{}, ErrParseFailed
	}
	return result, nil
}

// ParsePRSummary unmarshals the LLM's text response into PRSummaryResponse.
// Handles common LLM output quirks: markdown code fences, leading/trailing text.
func ParsePRSummary(raw string) (PRSummaryResponse, error) {
	s := cleanLLMJSON(raw)
	var result PRSummaryResponse
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return PRSummaryResponse{}, ErrParseFailed
	}
	return result, nil
}
