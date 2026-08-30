package scanner

import (
	"testing"
	"time"

	"inboxcleaner/internal/mailbox"
)

func TestParseHeaders(t *testing.T) {
	fallback := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		headers   map[string][]string
		wantList  string
		wantURL   string
		wantEmail string
		wantFrom  string
		wantDate  time.Time
	}{
		{
			name: "list ID and multiple unsubscribe methods",
			headers: map[string][]string{
				"from":             {"Morning Brew <crew@morningbrew.com>"},
				"date":             {"Fri, 28 Aug 2026 09:30:00 -0400"},
				"list-id":          {"Morning Brew <daily.morningbrew.com>"},
				"list-unsubscribe": {"<mailto:leave@morningbrew.com>, <https://MorningBrew.com/unsubscribe/abc#ignored>"},
			},
			wantList:  "daily.morningbrew.com",
			wantURL:   "https://morningbrew.com/unsubscribe/abc",
			wantEmail: "leave@morningbrew.com",
			wantFrom:  "crew@morningbrew.com",
			wantDate:  time.Date(2026, time.August, 28, 13, 30, 0, 0, time.UTC),
		},
		{
			name: "malformed headers use internal date",
			headers: map[string][]string{
				"from":             {"not an address"},
				"date":             {"yesterday-ish"},
				"list-unsubscribe": {"javascript:alert(1), definitely not a url"},
			},
			wantDate: fallback,
		},
		{
			name: "missing From is tolerated",
			headers: map[string][]string{
				"list-id": {"<news.example.com>"},
			},
			wantList: "news.example.com",
			wantDate: fallback,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ParseHeaders(mailbox.MessageMetadata{ID: "m1", Headers: test.headers, InternalDate: fallback})
			if got.ListID != test.wantList || got.UnsubscribeURL != test.wantURL || got.UnsubscribeEmail != test.wantEmail || got.Sender != test.wantFrom {
				t.Fatalf("unexpected parsed headers: %+v", got)
			}
			if !got.ReceivedAt.Equal(test.wantDate) {
				t.Fatalf("received date = %v, want %v", got.ReceivedAt, test.wantDate)
			}
		})
	}
}

func TestOneClickHeader(t *testing.T) {
	got := ParseHeaders(mailbox.MessageMetadata{Headers: map[string][]string{
		"list-unsubscribe-post": {"List-Unsubscribe=One-Click"},
	}})
	if !got.OneClick {
		t.Fatal("expected one-click header to be recognized")
	}
}
