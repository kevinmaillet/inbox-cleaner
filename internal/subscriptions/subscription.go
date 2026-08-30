package subscriptions

import (
	"sort"
	"time"
)

// Subscription is a logical mailing list or newsletter and its Gmail messages.
type Subscription struct {
	Key              string
	Name             string
	Sender           string
	SenderDomain     string
	ListID           string
	UnsubscribeURL   string
	UnsubscribeEmail string
	MessageIDs       []string
	MessageCount     int
	LastReceived     time.Time
}

// Sort ranks subscriptions by message count and uses the key as a stable tie-breaker.
func Sort(items []Subscription) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].MessageCount != items[j].MessageCount {
			return items[i].MessageCount > items[j].MessageCount
		}
		return items[i].Key < items[j].Key
	})
}
