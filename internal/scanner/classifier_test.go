package scanner

import "testing"

func TestIsSubscription(t *testing.T) {
	tests := []struct {
		name string
		h    ParsedHeaders
		want bool
	}{
		{"List-ID", ParsedHeaders{ListID: "list.example.com"}, true},
		{"HTTP unsubscribe only", ParsedHeaders{UnsubscribeURL: "https://example.com/u"}, true},
		{"mailto unsubscribe only", ParsedHeaders{UnsubscribeEmail: "leave@example.com"}, true},
		{"one click only", ParsedHeaders{OneClick: true}, true},
		{"bulk precedence", ParsedHeaders{Precedence: "bulk"}, true},
		{"list precedence", ParsedHeaders{Precedence: "list"}, true},
		{"weak signals together", ParsedHeaders{Sender: "newsletter@example.com", Subject: "Weekly digest"}, true},
		{"sender heuristic alone", ParsedHeaders{Sender: "newsletter@example.com", Subject: "Hello Kevin"}, false},
		{"ordinary personal email", ParsedHeaders{Sender: "friend@example.com", Subject: "Dinner tomorrow?"}, false},
		{"missing From and signals", ParsedHeaders{Subject: "Hello"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsSubscription(test.h); got != test.want {
				t.Fatalf("IsSubscription(%+v) = %v, want %v", test.h, got, test.want)
			}
		})
	}
}
