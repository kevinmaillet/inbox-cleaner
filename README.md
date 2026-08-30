# Inbox Cleaner

Licensed under the Apache License 2.0.

Inbox Cleaner is a local-first Gmail subscription scanner written in Go. It requests message metadata only, conservatively identifies newsletters and mailing lists, groups related messages, and ranks them by message count.

This first milestone is intentionally read-only. It does not download message bodies, trash mail, archive mail, follow unsubscribe links, or store derived email metadata.

## Prerequisites

- Go 1.23 or newer
- A Google Cloud project with the Gmail API enabled
- An OAuth 2.0 Desktop client credentials file

In Google Cloud Console, enable the Gmail API, configure the OAuth consent screen, and create an OAuth client with application type **Desktop app**. Download its JSON credentials.

Inbox Cleaner keeps credentials and tokens in the operating system's user config directory:

| OS | Default directory |
| --- | --- |
| macOS | `~/Library/Application Support/inboxcleaner/` |
| Linux | `~/.config/inboxcleaner/` (or `$XDG_CONFIG_HOME/inboxcleaner/`) |
| Windows | `%AppData%\\inboxcleaner\\` |

Copy the downloaded client JSON to `credentials.json` in that directory, or pass its current location with `--credentials`. The generated `token.json` is created with user-only permissions and must not be committed.

## Run

```bash
go run ./cmd/inboxcleaner scan
```

The first run prints an authorization URL and attempts to open it in your browser. The OAuth callback goes to a temporary loopback listener on `127.0.0.1`; the token remains on your machine.

Useful flags:

```bash
go run ./cmd/inboxcleaner scan --limit 10000
go run ./cmd/inboxcleaner scan --concurrency 4
go run ./cmd/inboxcleaner scan --query 'in:inbox newer_than:2y'
go run ./cmd/inboxcleaner scan -h
```

The default query is `in:anywhere -in:trash -in:spam`, the default limit is 10,000 messages, and the default metadata concurrency is 8. Pass `--limit 0` to scan every matching message.

## Architecture

- `cmd/inboxcleaner`: CLI composition and flags
- `internal/config`: platform-appropriate local paths
- `internal/gmail`: OAuth, Gmail REST pagination, bounded metadata fetching, and retries
- `internal/mailbox`: provider-neutral metadata streaming and mutation contracts
- `internal/scanner`: Gmail-independent parsing, classification, grouping, and scan service
- `internal/subscriptions`: domain model and deterministic ranking
- `internal/ui`: text table output

`mailbox.MetadataSource` is the read boundary and `mailbox.Mailbox` is the future trash/archive boundary. Gmail IDs are treated as opaque provider IDs. An Outlook, IMAP, or other adapter can implement the same contracts without changing classification, grouping, or UI code. Tests use a fake source, so `go test ./...` never needs OAuth credentials or an email account.

## Gmail access

OAuth scope:

```text
https://www.googleapis.com/auth/gmail.readonly
```

API methods used:

- `GET /gmail/v1/users/me/messages` to paginate message IDs
- `GET /gmail/v1/users/me/messages/{id}?format=metadata` to retrieve only selected headers and `internalDate`

Metadata requests select only `From`, `Subject`, `Date`, `List-ID`, `List-Unsubscribe`, `List-Unsubscribe-Post`, and `Precedence`. Transient network failures, HTTP 429 responses, and HTTP 5xx responses use bounded exponential backoff. Metadata fetch concurrency is bounded and configurable.

## Classification behavior

Strong signals are `List-ID`, a valid HTTP(S) or mailto `List-Unsubscribe` value, `List-Unsubscribe-Post: List-Unsubscribe=One-Click`, and `Precedence: list|bulk`. A sender-name heuristic is used only when paired with a newsletter-like subject signal.

Grouping prefers normalized `List-ID`, then normalized unsubscribe endpoint, then normalized sender email. A sender domain is only a last resort. This prevents two newsletters on the same provider domain from being merged when they have different list identifiers.

Known limitations:

- Senders that omit mailing-list headers and do not match the narrow fallback heuristic are intentionally missed.
- Incorrect or deceptive headers can still produce a false positive.
- Display names can change; the newest parsed display name wins.
- The default query counts messages outside the Inbox (except Trash and Spam), so use `--query in:inbox` if "inbox space" should mean the Inbox label literally.
- Results are not cached yet, so repeated scans repeat metadata calls.

## Tests

```bash
go test ./...
```

Tests cover malformed and missing headers, `List-ID`, multiple unsubscribe values, HTTP and mailto unsubscribe methods, one-click headers, conservative classification, unrelated lists on the same domain, deterministic sorting, retained Gmail message IDs, and ordinary personal mail.

GitHub Actions verifies formatting and runs the race-enabled unit test suite on pull requests and pushes to `main`.

## Next milestone: safe cleanup

Bulk trash/archive should add the `gmail.modify` scope, persist scan results by subscription key, re-resolve message IDs before acting, show the exact count and subscription name, require confirmation unless `--yes` is present, and use Gmail batch modification to add/remove system labels. Trash must add `TRASH`; archive must remove `INBOX`. Permanent deletion should remain out of scope.
