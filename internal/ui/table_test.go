package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"inboxcleaner/internal/subscriptions"
)

func TestPrintSubscriptions(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	items := []subscriptions.Subscription{{Name: "Morning Brew", MessageCount: 184, LastReceived: now}}
	var output bytes.Buffer
	if err := PrintSubscriptions(&output, items, now); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"COUNT", "LAST RECEIVED", "SUBSCRIPTION", "184", "today", "Morning Brew"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("output %q does not contain %q", output.String(), expected)
		}
	}
}
