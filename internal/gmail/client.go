package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"inboxcleaner/internal/mailbox"
)

const apiBaseURL = "https://gmail.googleapis.com/gmail/v1/users/me"

var metadataHeaders = []string{
	"From", "Subject", "Date", "List-ID", "List-Unsubscribe",
	"List-Unsubscribe-Post", "Precedence",
}

type Client struct {
	httpClient  *http.Client
	query       string
	concurrency int
	maxRetries  int
	baseURL     string
}

var _ mailbox.MetadataSource = (*Client)(nil)

func NewClient(httpClient *http.Client, query string, concurrency int) *Client {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Client{
		httpClient:  httpClient,
		query:       query,
		concurrency: concurrency,
		maxRetries:  5,
		baseURL:     apiBaseURL,
	}
}

type messageRef struct {
	ID string `json:"id"`
}

type listResponse struct {
	Messages      []messageRef `json:"messages"`
	NextPageToken string       `json:"nextPageToken"`
}

type getResponse struct {
	ID           string `json:"id"`
	InternalDate string `json:"internalDate"`
	Payload      struct {
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"payload"`
}

type workResult struct {
	message mailbox.MessageMetadata
	err     error
}

// StreamMetadata paginates IDs, fetches metadata concurrently, and calls fn serially.
func (c *Client) StreamMetadata(ctx context.Context, limit int, fn func(mailbox.MessageMetadata) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan string, c.concurrency*2)
	results := make(chan workResult, c.concurrency*2)

	var workers sync.WaitGroup
	for i := 0; i < c.concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range jobs {
				message, err := c.getMessage(ctx, id)
				select {
				case results <- workResult{message: message, err: err}:
				case <-ctx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		if err := c.listMessageIDs(ctx, limit, jobs); err != nil {
			select {
			case results <- workResult{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	for result := range results {
		if result.err != nil {
			cancel()
			return result.err
		}
		if err := fn(result.message); err != nil {
			cancel()
			return err
		}
	}
	return ctx.Err()
}

func (c *Client) listMessageIDs(ctx context.Context, limit int, jobs chan<- string) error {
	pageToken := ""
	sent := 0
	for {
		pageSize := 500
		if limit > 0 && limit-sent < pageSize {
			pageSize = limit - sent
		}
		if pageSize <= 0 {
			return nil
		}
		params := url.Values{}
		params.Set("maxResults", strconv.Itoa(pageSize))
		params.Set("fields", "messages/id,nextPageToken")
		if c.query != "" {
			params.Set("q", c.query)
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		var page listResponse
		if err := c.getJSON(ctx, c.baseURL+"/messages?"+params.Encode(), &page); err != nil {
			return fmt.Errorf("list Gmail messages: %w", err)
		}
		for _, message := range page.Messages {
			select {
			case jobs <- message.ID:
				sent++
			case <-ctx.Done():
				return ctx.Err()
			}
			if limit > 0 && sent >= limit {
				return nil
			}
		}
		if page.NextPageToken == "" {
			return nil
		}
		pageToken = page.NextPageToken
	}
}

func (c *Client) getMessage(ctx context.Context, id string) (mailbox.MessageMetadata, error) {
	params := url.Values{}
	params.Set("format", "metadata")
	params.Set("fields", "id,internalDate,payload/headers")
	for _, header := range metadataHeaders {
		params.Add("metadataHeaders", header)
	}
	var message getResponse
	rawURL := c.baseURL + "/messages/" + url.PathEscape(id) + "?" + params.Encode()
	if err := c.getJSON(ctx, rawURL, &message); err != nil {
		return mailbox.MessageMetadata{}, fmt.Errorf("get Gmail message %s: %w", id, err)
	}

	headers := make(map[string][]string, len(message.Payload.Headers))
	for _, header := range message.Payload.Headers {
		name := strings.ToLower(strings.TrimSpace(header.Name))
		if name != "" {
			headers[name] = append(headers[name], header.Value)
		}
	}
	var internalDate time.Time
	if millis, err := strconv.ParseInt(message.InternalDate, 10, 64); err == nil {
		internalDate = time.UnixMilli(millis)
	}
	return mailbox.MessageMetadata{ID: message.ID, Headers: headers, InternalDate: internalDate}, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, destination any) error {
	for attempt := 0; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		response, err := c.httpClient.Do(request)
		if err == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
			defer response.Body.Close()
			if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
				return fmt.Errorf("decode Gmail response: %w", err)
			}
			return nil
		}

		retryable := err != nil
		var status string
		if response != nil {
			status = response.Status
			retryable = response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
			if !retryable {
				body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
				response.Body.Close()
				return fmt.Errorf("Gmail API returned %s: %s", status, strings.TrimSpace(string(body)))
			}
			response.Body.Close()
		}
		if !retryable || attempt >= c.maxRetries {
			if err != nil {
				return err
			}
			return fmt.Errorf("Gmail API returned %s after retries", status)
		}
		delay := 250 * time.Millisecond * time.Duration(1<<attempt)
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}
