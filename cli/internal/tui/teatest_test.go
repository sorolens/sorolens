package tui

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// TestTeatestDashboardRendersAndQuits drives the dashboard through the same
// headless terminal driver used by bubbletea's own integration tests.
func TestTeatestDashboardRendersAndQuits(t *testing.T) {
	m := New(newFakeAPI(), Options{
		APIURL:          "http://localhost:8080",
		RefreshInterval: time.Hour,
		RequestTimeout:  time.Second,
	})

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 60))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Storage TTL")) &&
			bytes.Contains(b, []byte("alpha")) &&
			bytes.Contains(b, []byte("transfer"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
