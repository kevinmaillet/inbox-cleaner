package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/kevinmaillet/inbox-cleaner/internal/mailbox"
)

func TestStreamMetadataPaginatesAndFetchesMetadataOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/messages":
			if r.URL.Query().Get("q") != "in:inbox" {
				t.Errorf("q = %q, want in:inbox", r.URL.Query().Get("q"))
			}
			if r.URL.Query().Get("pageToken") == "next" {
				writeJSON(t, w, map[string]any{"messages": []map[string]string{{"id": "c"}}})
				return
			}
			writeJSON(t, w, map[string]any{
				"messages":      []map[string]string{{"id": "a"}, {"id": "b"}},
				"nextPageToken": "next",
			})
		case "/messages/a", "/messages/b", "/messages/c":
			if r.URL.Query().Get("format") != "metadata" {
				t.Errorf("format = %q, want metadata", r.URL.Query().Get("format"))
			}
			if got := r.URL.Query()["metadataHeaders"]; len(got) != len(metadataHeaders) {
				t.Errorf("metadataHeaders = %v, want %v", got, metadataHeaders)
			}
			id := r.URL.Path[len("/messages/"):]
			writeJSON(t, w, map[string]any{
				"id":           id,
				"internalDate": "1787920200000",
				"payload": map[string]any{"headers": []map[string]string{
					{"name": "From", "value": "News <news@example.com>"},
					{"name": "List-ID", "value": "<news.example.com>"},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client(), "in:inbox", 3)
	client.baseURL = server.URL
	client.maxRetries = 0
	var messages []mailbox.MessageMetadata
	err := client.StreamMetadata(context.Background(), 0, func(message mailbox.MessageMetadata) error {
		messages = append(messages, message)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
		if message.Headers["from"][0] != "News <news@example.com>" {
			t.Fatalf("headers were not normalized: %+v", message.Headers)
		}
	}
	sort.Strings(ids)
	if fmt.Sprint(ids) != "[a b c]" {
		t.Fatalf("IDs = %v, want [a b c]", ids)
	}
}

func TestGetJSONRetriesRateLimit(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.Client(), "", 1)
	client.maxRetries = 1
	var response struct {
		Status string `json:"status"`
	}
	if err := client.getJSON(context.Background(), server.URL, &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "ok" || attempts.Load() != 2 {
		t.Fatalf("response=%+v attempts=%d", response, attempts.Load())
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
