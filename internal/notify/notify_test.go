package notify

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func fakeLookPath(available map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if path, ok := available[name]; ok {
			return path, nil
		}
		return "", errors.New("not found")
	}
}

func TestSendWithTerminalNotifier(t *testing.T) {
	var calledBin string
	var calledArgs []string
	run := func(name string, args ...string) error {
		calledBin = name
		calledArgs = args
		return nil
	}

	n := Notification{
		Title:    "gh-dash",
		Subtitle: "PR #42",
		Message:  "review requested",
		OpenURL:  "https://github.com/org/repo/pull/42",
	}
	sendWith(
		n,
		fakeLookPath(map[string]string{"terminal-notifier": "/usr/local/bin/terminal-notifier"}),
		run,
	)

	require.Equal(t, "/usr/local/bin/terminal-notifier", calledBin)
	require.Contains(t, calledArgs, "-title")
	require.Contains(t, calledArgs, "gh-dash")
	require.Contains(t, calledArgs, "-subtitle")
	require.Contains(t, calledArgs, "PR #42")
	require.Contains(t, calledArgs, "-open")
	require.Contains(t, calledArgs, "https://github.com/org/repo/pull/42")
}

func TestSendWithOsascriptFallback(t *testing.T) {
	var calledBin string
	var calledArgs []string
	run := func(name string, args ...string) error {
		calledBin = name
		calledArgs = args
		return nil
	}

	n := Notification{Title: "gh-dash", Message: "review requested"}
	sendWith(n, fakeLookPath(map[string]string{"osascript": "/usr/bin/osascript"}), run)

	require.Equal(t, "/usr/bin/osascript", calledBin)
	require.Equal(t, "-e", calledArgs[0])
	require.Contains(t, calledArgs[1], "gh-dash")
	require.Contains(t, calledArgs[1], "review requested")
}

func TestSendWithSubtitleInOsascriptFallback(t *testing.T) {
	var calledArgs []string
	run := func(_ string, args ...string) error {
		calledArgs = args
		return nil
	}

	n := Notification{Title: "gh-dash", Subtitle: "PR #5", Message: "new comments"}
	sendWith(n, fakeLookPath(map[string]string{"osascript": "/usr/bin/osascript"}), run)

	// subtitle is prepended to message body in osascript fallback
	require.Contains(t, calledArgs[1], "PR #5")
	require.Contains(t, calledArgs[1], "new comments")
}

func TestSendWithNoBackendAvailable(t *testing.T) {
	called := false
	run := func(_ string, _ ...string) error {
		called = true
		return nil
	}

	n := Notification{Title: "gh-dash", Message: "test"}
	sendWith(n, fakeLookPath(map[string]string{}), run)
	require.False(t, called, "run should not be called when no backend is available")
}

func TestQuote(t *testing.T) {
	require.Equal(t, `"hello"`, quote("hello"))
	require.Equal(t, `"say \"hi\""`, quote(`say "hi"`))
}
