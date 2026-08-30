// Package mailbox defines provider-neutral email capabilities used by the application.
package mailbox

import (
	"context"
	"time"
)

// MessageMetadata contains only fields needed to classify and group a message.
// ID is opaque and meaningful only to the provider that returned it.
type MessageMetadata struct {
	ID           string
	Headers      map[string][]string
	InternalDate time.Time
}

// MetadataSource streams metadata in bounded memory. Implementations must invoke fn serially.
// Provider-specific filtering belongs in the implementation's configuration.
type MetadataSource interface {
	StreamMetadata(ctx context.Context, limit int, fn func(MessageMetadata) error) error
}

// Mailbox is the provider-neutral mutation boundary for the next milestone.
// Implementations must treat message IDs as opaque and must not permanently delete mail.
type Mailbox interface {
	Trash(ctx context.Context, messageIDs []string) error
	Archive(ctx context.Context, messageIDs []string) error
}
