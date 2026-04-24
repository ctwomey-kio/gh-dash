package notify

import (
	"os/exec"
	"strings"

	"charm.land/log/v2"
)

// Notification holds the visible slots of a macOS desktop notification.
type Notification struct {
	Title    string
	Subtitle string
	Message  string
	Group    string
	OpenURL  string
}

// Send fires a desktop notification asynchronously. It prefers terminal-notifier
// (supports -subtitle and click-to-open) and falls back to osascript.
func Send(n Notification) {
	go sendWith(n, exec.LookPath, func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	})
}

// sendWith is the testable core of Send. lookPath resolves binary names;
// run executes the binary. Both are injected so tests can avoid shelling out.
func sendWith(
	n Notification,
	lookPath func(string) (string, error),
	run func(string, ...string) error,
) {
	if tn, err := lookPath("terminal-notifier"); err == nil {
		group := n.Group
		if group == "" {
			group = "gh-dash"
		}
		args := []string{
			"-title", n.Title,
			"-message", n.Message,
			"-group", group,
		}
		if n.Subtitle != "" {
			args = append(args, "-subtitle", n.Subtitle)
		}
		if n.OpenURL != "" {
			args = append(args, "-open", n.OpenURL)
		}
		if err := run(tn, args...); err != nil {
			log.Error("terminal-notifier failed", "err", err)
		}
		return
	}
	// Fall back to osascript (no subtitle support)
	body := n.Message
	if n.Subtitle != "" {
		body = n.Subtitle + "\n" + n.Message
	}
	script := "display notification " + quote(body) + " with title " + quote(n.Title)
	if osascript, err := lookPath("osascript"); err == nil {
		if err := run(osascript, "-e", script); err != nil {
			log.Error("osascript notification failed", "err", err)
		}
	}
}

// quote wraps s in a valid AppleScript string literal. Backslashes and double
// quotes are escaped; newlines become AppleScript linefeed concatenation since
// literal newlines break the string literal boundary.
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\r\n", `" & linefeed & "`)
	s = strings.ReplaceAll(s, "\n", `" & linefeed & "`)
	s = strings.ReplaceAll(s, "\r", `" & linefeed & "`)
	return `"` + s + `"`
}
