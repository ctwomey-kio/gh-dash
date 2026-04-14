package notify

import (
	"os/exec"

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
	go func() {
		if tn, err := exec.LookPath("terminal-notifier"); err == nil {
			args := []string{
				"-title", n.Title,
				"-message", n.Message,
				"-group", "gh-dash",
			}
			if n.Subtitle != "" {
				args = append(args, "-subtitle", n.Subtitle)
			}
			if n.OpenURL != "" {
				args = append(args, "-open", n.OpenURL)
			}
			if err := exec.Command(tn, args...).Run(); err != nil {
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
		if err := exec.Command("osascript", "-e", script).Run(); err != nil {
			log.Error("osascript notification failed", "err", err)
		}
	}()
}

// quote wraps s in double quotes and escapes internal double quotes for AppleScript.
func quote(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			out = append(out, '\\', '"')
		} else {
			out = append(out, s[i])
		}
	}
	out = append(out, '"')
	return string(out)
}
