package scanner

import "strings"

// IsSubscription deliberately prefers false negatives over personal-email false positives.
func IsSubscription(h ParsedHeaders) bool {
	if h.ListID != "" || h.UnsubscribeURL != "" || h.UnsubscribeEmail != "" || h.OneClick {
		return true
	}
	if h.Precedence == "list" || h.Precedence == "bulk" {
		return true
	}

	// A weak sender signal is accepted only with a second subject signal.
	local := h.Sender
	if at := strings.IndexByte(local, '@'); at >= 0 {
		local = local[:at]
	}
	weakSender := strings.Contains(local, "newsletter") || strings.Contains(local, "digest")
	subject := strings.ToLower(strings.TrimSpace(h.Subject))
	weakSubject := strings.HasPrefix(subject, "weekly ") || strings.HasPrefix(subject, "daily ") ||
		strings.Contains(subject, " newsletter") || strings.Contains(subject, " digest")
	return weakSender && weakSubject
}
