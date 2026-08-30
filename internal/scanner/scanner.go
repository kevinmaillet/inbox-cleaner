package scanner

import (
	"context"
	"fmt"

	"inboxcleaner/internal/mailbox"
	"inboxcleaner/internal/subscriptions"
)

type Service struct {
	source mailbox.MetadataSource
	limit  int
}

func New(source mailbox.MetadataSource, limit int) *Service {
	return &Service{source: source, limit: limit}
}

func (s *Service) Scan(ctx context.Context) ([]subscriptions.Subscription, error) {
	g := newGrouper()
	err := s.source.StreamMetadata(ctx, s.limit, func(message mailbox.MessageMetadata) error {
		if err := ValidateMessage(message); err != nil {
			return err
		}
		headers := ParseHeaders(message)
		if IsSubscription(headers) {
			g.add(message, headers)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan mailbox: %w", err)
	}
	return g.subscriptions(), nil
}
