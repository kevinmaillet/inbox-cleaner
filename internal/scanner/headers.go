package scanner

import (
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/kevinmaillet/inbox-cleaner/internal/mailbox"
)

// ParsedHeaders is the normalized subset used by classification and grouping.
type ParsedHeaders struct {
	DisplayName      string
	Sender           string
	SenderDomain     string
	Subject          string
	ListID           string
	UnsubscribeURL   string
	UnsubscribeEmail string
	Precedence       string
	OneClick         bool
	ReceivedAt       time.Time
}

// ParseHeaders tolerates missing and malformed headers and never downloads a body.
func ParseHeaders(message mailbox.MessageMetadata) ParsedHeaders {
	value := func(name string) string {
		values := headerValues(message.Headers, name)
		if len(values) == 0 {
			return ""
		}
		return strings.TrimSpace(values[0])
	}

	parsed := ParsedHeaders{
		Subject:    value("Subject"),
		ListID:     normalizeListID(value("List-ID")),
		Precedence: strings.ToLower(value("Precedence")),
		OneClick: strings.EqualFold(
			strings.TrimSpace(value("List-Unsubscribe-Post")),
			"List-Unsubscribe=One-Click",
		),
		ReceivedAt: message.InternalDate,
	}

	if address, err := mail.ParseAddress(value("From")); err == nil {
		parsed.DisplayName = strings.TrimSpace(address.Name)
		parsed.Sender = strings.ToLower(strings.TrimSpace(address.Address))
		if at := strings.LastIndexByte(parsed.Sender, '@'); at >= 0 && at+1 < len(parsed.Sender) {
			parsed.SenderDomain = strings.TrimSuffix(parsed.Sender[at+1:], ".")
		}
	}

	if date, err := mail.ParseDate(value("Date")); err == nil {
		parsed.ReceivedAt = date
	}
	parsed.UnsubscribeURL, parsed.UnsubscribeEmail = parseUnsubscribe(headerValues(message.Headers, "List-Unsubscribe"))
	return parsed
}

func headerValues(headers map[string][]string, name string) []string {
	if values := headers[strings.ToLower(name)]; len(values) > 0 {
		return values
	}
	for key, values := range headers {
		if strings.EqualFold(key, name) {
			return values
		}
	}
	return nil
}

func normalizeListID(raw string) string {
	raw = strings.TrimSpace(raw)
	if open := strings.LastIndexByte(raw, '<'); open >= 0 {
		if close := strings.IndexByte(raw[open+1:], '>'); close >= 0 {
			raw = raw[open+1 : open+1+close]
		}
	}
	return strings.ToLower(strings.Trim(strings.TrimSpace(raw), "<>"))
}

func parseUnsubscribe(values []string) (httpURL, email string) {
	var candidates []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			candidate := strings.Trim(strings.TrimSpace(part), "<>")
			if candidate != "" {
				candidates = append(candidates, candidate)
			}
		}
	}

	// Sort only to make malformed/duplicate header input deterministic.
	sort.Strings(candidates)
	for _, candidate := range candidates {
		u, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		switch strings.ToLower(u.Scheme) {
		case "https", "http":
			if httpURL == "" && u.Host != "" {
				u.Scheme = strings.ToLower(u.Scheme)
				u.Host = strings.ToLower(u.Host)
				u.Fragment = ""
				httpURL = u.String()
			}
		case "mailto":
			if email == "" {
				address := strings.ToLower(strings.TrimSpace(u.Opaque))
				if parsed, err := mail.ParseAddress(address); err == nil {
					email = strings.ToLower(parsed.Address)
				}
			}
		}
	}
	return httpURL, email
}
