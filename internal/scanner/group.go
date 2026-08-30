package scanner

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/kevinmaillet/inbox-cleaner/internal/mailbox"
	"github.com/kevinmaillet/inbox-cleaner/internal/subscriptions"
)

type grouper struct {
	items  map[string]*subscriptions.Subscription
	stamps map[string]*fieldStamps
}

type stamp struct {
	time string
	id   string
}

type fieldStamps struct {
	name   stamp
	sender stamp
	url    stamp
	email  stamp
}

func newGrouper() *grouper {
	return &grouper{
		items:  make(map[string]*subscriptions.Subscription),
		stamps: make(map[string]*fieldStamps),
	}
}

func subscriptionKey(h ParsedHeaders) string {
	if h.ListID != "" {
		return "list:" + h.ListID
	}
	if h.UnsubscribeURL != "" {
		if u, err := url.Parse(h.UnsubscribeURL); err == nil {
			u.Fragment = ""
			return "unsubscribe:" + u.String()
		}
	}
	if h.UnsubscribeEmail != "" {
		return "unsubscribe:mailto:" + h.UnsubscribeEmail
	}
	if h.Sender != "" {
		return "sender:" + h.Sender
	}
	if h.SenderDomain != "" {
		return "domain:" + h.SenderDomain
	}
	return ""
}

func (g *grouper) add(message mailbox.MessageMetadata, h ParsedHeaders) {
	key := subscriptionKey(h)
	if key == "" {
		return
	}
	item := g.items[key]
	if item == nil {
		name := h.DisplayName
		if name == "" {
			name = h.ListID
		}
		if name == "" {
			name = h.Sender
		}
		if name == "" {
			name = key
		}
		item = &subscriptions.Subscription{
			Key:              key,
			Name:             name,
			Sender:           h.Sender,
			SenderDomain:     h.SenderDomain,
			ListID:           h.ListID,
			UnsubscribeURL:   h.UnsubscribeURL,
			UnsubscribeEmail: h.UnsubscribeEmail,
		}
		g.items[key] = item
		g.stamps[key] = &fieldStamps{}
	}
	stamps := g.stamps[key]
	messageStamp := stamp{time: h.ReceivedAt.UTC().Format("20060102T150405.000000000Z07:00"), id: message.ID}
	item.MessageIDs = append(item.MessageIDs, message.ID)
	item.MessageCount++
	if h.ReceivedAt.After(item.LastReceived) {
		item.LastReceived = h.ReceivedAt
	}
	if h.DisplayName != "" && newer(messageStamp, stamps.name) {
		item.Name = h.DisplayName
		stamps.name = messageStamp
	}
	if h.Sender != "" && newer(messageStamp, stamps.sender) {
		item.Sender = h.Sender
		item.SenderDomain = h.SenderDomain
		stamps.sender = messageStamp
	}
	if h.UnsubscribeURL != "" && newer(messageStamp, stamps.url) {
		item.UnsubscribeURL = h.UnsubscribeURL
		stamps.url = messageStamp
	}
	if h.UnsubscribeEmail != "" && newer(messageStamp, stamps.email) {
		item.UnsubscribeEmail = h.UnsubscribeEmail
		stamps.email = messageStamp
	}
}

func newer(candidate, current stamp) bool {
	return candidate.time > current.time || (candidate.time == current.time && candidate.id > current.id)
}

func (g *grouper) subscriptions() []subscriptions.Subscription {
	items := make([]subscriptions.Subscription, 0, len(g.items))
	for _, item := range g.items {
		copy := *item
		sort.Strings(copy.MessageIDs)
		items = append(items, copy)
	}
	subscriptions.Sort(items)
	return items
}

// ValidateMessage provides a useful error for impossible fake/source output.
func ValidateMessage(message mailbox.MessageMetadata) error {
	if strings.TrimSpace(message.ID) == "" {
		return fmt.Errorf("message metadata is missing an ID")
	}
	return nil
}
