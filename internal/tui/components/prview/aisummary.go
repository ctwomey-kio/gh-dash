package prview

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/dlvhdr/gh-dash/v4/internal/ai"
)

// Package-level compiled regexes for extracting complete JSON string fields during streaming.
var (
	reCategory     = regexp.MustCompile(`"category"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	reInterest     = regexp.MustCompile(`"interest"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	reSummaryField = regexp.MustCompile(`"summary"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	reReviewStatus = regexp.MustCompile(`"review_status"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	reRiskNotes    = regexp.MustCompile(`"risk_notes"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	reArrayItem    = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
)

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// extractStringField pulls a complete JSON string value for the given field regex.
// Returns empty string if the field hasn't finished streaming yet.
func extractStringField(s string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	unq, err := strconv.Unquote(`"` + m[1] + `"`)
	if err != nil {
		return m[1]
	}
	return unq
}

// extractPartialStringArray returns all complete string items found so far in a
// streaming JSON array, even if the array hasn't closed yet.
func extractPartialStringArray(s, field string) []string {
	marker := `"` + field + `"`
	idx := strings.Index(s, marker)
	if idx == -1 {
		return nil
	}
	after := s[idx+len(marker):]
	bracketIdx := strings.Index(after, "[")
	if bracketIdx == -1 {
		return nil
	}
	content := after[bracketIdx+1:]
	// Trim to closing bracket if present (complete array), otherwise use all we have
	if endIdx := strings.Index(content, "]"); endIdx != -1 {
		content = content[:endIdx]
	}
	matches := reArrayItem.FindAllStringSubmatch(content, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if unq, err := strconv.Unquote(`"` + m[1] + `"`); err == nil {
			result = append(result, unq)
		} else {
			result = append(result, m[1])
		}
	}
	return result
}

// parsePartialAccum extracts whatever complete fields are present in a partially-streamed
// JSON response. Incomplete fields are left as zero values.
func parsePartialAccum(accum string) ai.PRSummaryResponse {
	s := strings.TrimPrefix(accum, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(s)

	r := ai.PRSummaryResponse{}
	r.Category = extractStringField(s, reCategory)
	r.Interest = extractStringField(s, reInterest)
	r.Summary = extractStringField(s, reSummaryField)
	r.KeyFiles = extractPartialStringArray(s, "key_files")
	r.ReviewStatus = extractStringField(s, reReviewStatus)
	if rn := extractStringField(s, reRiskNotes); rn != "" {
		r.RiskNotes = &rn
	}
	return r
}

func (m *Model) renderAISummary() string {
	w := m.getIndentedContentWidth()
	faint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	heading := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true)
	body := lipgloss.NewStyle().Width(w)

	if m.pr == nil || !m.pr.Data.IsEnriched {
		return faint.Render("Waiting for PR data...")
	}

	if m.ctx == nil || m.ctx.AIClient == nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			faint.Render("AI summary is not configured."),
			"",
			faint.Render("To enable, set ANTHROPIC_API_KEY and add to config.yml:"),
			faint.Render("  ai:"),
			faint.Render("    enabled: true"),
		)
	}

	if m.aiSummaryLoading {
		if m.aiStreamAccum != "" {
			partial := parsePartialAccum(m.aiStreamAccum)
			return m.renderSummaryFields(partial, true, 0, faint, heading, body)
		}
		return faint.Render("Generating AI summary...")
	}

	if m.aiSummaryError != nil {
		errStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText)
		return errStyle.Render(fmt.Sprintf("Error: %s", m.aiSummaryError.Error()))
	}

	if m.aiSummary == nil {
		return faint.Render("No summary available.")
	}

	return m.renderSummaryFields(*m.aiSummary, false, m.aiSummaryDuration, faint, heading, body)
}

// renderSummaryFields renders a PRSummaryResponse in structured form.
// isPartial=true adds a ▌ cursor and skips duration; missing fields are silently omitted.
func (m *Model) renderSummaryFields(s ai.PRSummaryResponse, isPartial bool, duration time.Duration, faint, heading, body lipgloss.Style) string {
	var out strings.Builder

	// Interest badge + category — render as soon as either field arrives
	if s.Interest != "" || s.Category != "" {
		badgeStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
		switch s.Interest {
		case "HIGH":
			badgeStyle = badgeStyle.Background(m.ctx.Theme.ErrorText).Foreground(lipgloss.Color("#ffffff"))
		case "MED":
			badgeStyle = badgeStyle.Background(m.ctx.Theme.WarningText).Foreground(lipgloss.Color("#000000"))
		default:
			badgeStyle = badgeStyle.Background(m.ctx.Theme.SuccessText).Foreground(lipgloss.Color("#000000"))
		}
		catStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).MarginLeft(1)
		durationStr := ""
		if !isPartial && duration > 0 {
			durationStr = " · " + formatDuration(duration)
		}
		out.WriteString(lipgloss.JoinHorizontal(lipgloss.Center,
			badgeStyle.Render(s.Interest),
			catStyle.Render(s.Category+durationStr),
		))
		out.WriteString("\n\n")
	}

	// Summary — render once the full string value is complete
	if s.Summary != "" {
		out.WriteString(heading.Render(" Summary"))
		out.WriteString("\n")
		out.WriteString(body.Foreground(m.ctx.Theme.SecondaryText).Render(s.Summary))
		out.WriteString("\n\n")
	}

	// Key files — render items as they complete, even mid-array
	if len(s.KeyFiles) > 0 {
		out.WriteString(heading.Render(" Key Files"))
		out.WriteString("\n")
		for _, f := range s.KeyFiles {
			out.WriteString(faint.Render("  " + f))
			out.WriteString("\n")
		}
		out.WriteString("\n")
	}

	// Review status
	if s.ReviewStatus != "" {
		out.WriteString(heading.Render(" Review Status"))
		out.WriteString("\n")
		out.WriteString(body.Foreground(m.ctx.Theme.SecondaryText).Render(s.ReviewStatus))
		out.WriteString("\n")
	}

	// Risk notes (optional)
	if s.RiskNotes != nil && *s.RiskNotes != "" {
		out.WriteString("\n")
		warnStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Bold(true)
		out.WriteString(warnStyle.Render("⚠ Risk Notes"))
		out.WriteString("\n")
		out.WriteString(body.Foreground(m.ctx.Theme.SecondaryText).Render(*s.RiskNotes))
	}

	// Cursor while generating
	if isPartial {
		if out.Len() == 0 {
			return faint.Render("Generating AI summary... ▌")
		}
		out.WriteString("\n")
		out.WriteString(faint.Render("▌"))
	}

	if out.Len() == 0 {
		return faint.Render("No summary available.")
	}

	return out.String()
}
