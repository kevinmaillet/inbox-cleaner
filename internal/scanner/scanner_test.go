package scanner

import (
	"context"
	"testing"
	"time"

	"inboxcleaner/internal/mailbox"
)

type fakeSource struct {
	messages []mailbox.MessageMetadata
}

func (f fakeSource) StreamMetadata(ctx context.Context, limit int, fn func(mailbox.MessageMetadata) error) error {
	for i, message := range f.messages {
		if limit > 0 && i >= limit {
			break
		}
		if err := fn(message); err != nil {
			return err
		}
	}
	return nil
}

func TestScanGroupsAndSorts(t *testing.T) {
	date := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	source := fakeSource{messages: []mailbox.MessageMetadata{
		message("a2", "Alpha Latest <news@shared.example>", "<alpha.shared.example>", "", date.Add(time.Hour)),
		message("a1", "Alpha Old <news@shared.example>", "<alpha.shared.example>", "", date),
		message("b1", "Beta <news@shared.example>", "<beta.shared.example>", "", date),
		message("c1", "Updates <updates@another.example>", "", "<https://another.example/leave/123>", date),
		message("personal", "Friend <friend@example.com>", "", "", date),
	}}

	items, err := New(source, 0).Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d subscriptions, want 3: %+v", len(items), items)
	}
	if items[0].Key != "list:alpha.shared.example" || items[0].MessageCount != 2 {
		t.Fatalf("first subscription = %+v", items[0])
	}
	if items[1].Key != "list:beta.shared.example" {
		t.Fatalf("same domain with different List-ID was grouped: %+v", items)
	}
	if len(items[0].MessageIDs) != 2 || items[0].MessageIDs[0] != "a1" || items[0].MessageIDs[1] != "a2" {
		t.Fatalf("message IDs were not retained: %+v", items[0].MessageIDs)
	}
	if items[0].Name != "Alpha Latest" {
		t.Fatalf("newest display name was not selected deterministically: %q", items[0].Name)
	}
}

func message(id, from, listID, unsubscribe string, date time.Time) mailbox.MessageMetadata {
	headers := map[string][]string{"from": {from}, "date": {date.Format(time.RFC1123Z)}}
	if listID != "" {
		headers["list-id"] = []string{listID}
	}
	if unsubscribe != "" {
		headers["list-unsubscribe"] = []string{unsubscribe}
	}
	return mailbox.MessageMetadata{ID: id, Headers: headers, InternalDate: date}
}
