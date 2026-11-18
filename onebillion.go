// Package onebillion is the Go client of 1billion: transactional e-mail on its own sending
// infrastructure.
// Use it only on a server: an API key must never reach a browser.
//
//	client := onebillion.New(os.Getenv("ONEBILLION_KEY"))
//
//	_, err := client.Send(ctx, onebillion.Mail{
//		From:    "App <hi@app.com>",
//		To:      []string{"ana@example.com"},
//		Subject: "Seu código",
//		Text:    "482 913",
//	})
//
// A 503 or a network error is tried again (3 times in all) with the same Idempotency-Key, so a
// retry never sends twice. No other error is tried again.
package onebillion

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Version is the version of this client, in its User-Agent.
const Version = "0.2.0"

// DefaultURL is the public API.
const DefaultURL = "https://1billion.clowk.in"

const tries = 3

// Client calls the API with the key of one app.
type Client struct {
	key     string
	url     string
	http    *http.Client
	backoff time.Duration
}

// Option changes a Client.
type Option func(*Client)

// WithURL points the client to another console, as the dev stack.
func WithURL(u string) Option { return func(c *Client) { c.url = u } }

// WithHTTPClient replaces the HTTP client (10 s timeout by default).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithBackoff sets the first wait between tries; it doubles each time.
func WithBackoff(d time.Duration) Option { return func(c *Client) { c.backoff = d } }

// New makes a client with an API key of an app.
func New(key string, opts ...Option) *Client {
	c := &Client{key: key, url: DefaultURL, http: &http.Client{Timeout: 10 * time.Second}, backoff: 500 * time.Millisecond}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Mail is the body of POST /send. Text, HTML or both; Raw sends a finished MIME message.
type Mail struct {
	From        string       `json:"from,omitempty"`
	To          []string     `json:"to,omitempty"`
	Cc          []string     `json:"cc,omitempty"`
	Bcc         []string     `json:"bcc,omitempty"`
	ReplyTo     string       `json:"reply_to,omitempty"`
	Subject     string       `json:"subject,omitempty"`
	Text        string       `json:"-"`
	HTML        string       `json:"-"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Priority    string       `json:"priority,omitempty"`
	Raw         string       `json:"raw,omitempty"`
}

// Attachment is one file; Content is base64.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Content     string `json:"content"`
}

// Accepted is the answer of a send.
type Accepted struct {
	Messages []struct {
		ID     string `json:"id"`
		To     string `json:"to"`
		Status string `json:"status"`
	} `json:"messages"`
}

// Message is the state of one recipient.
type Message struct {
	ID        string    `json:"id"`
	To        string    `json:"to"`
	Status    string    `json:"status"`
	Attempts  int       `json:"attempts"`
	LastError string    `json:"last_error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validation is the answer of POST /validate.
type Validation struct {
	Address string `json:"address"`
	User    string `json:"user"`
	Domain  string `json:"domain"`

	// The verdict: deliverable, risky or undeliverable; why; and a score from 0 to 1.
	Result string  `json:"result"`
	Reason string  `json:"reason"`
	Score  float64 `json:"score"`

	// The signals.
	DidYouMean  *string `json:"did_you_mean"`
	Free        bool    `json:"free"`
	Role        bool    `json:"role"`
	Disposable  bool    `json:"disposable"`
	Generated   bool    `json:"generated"`
	Subaddress  bool    `json:"subaddress"`
	BaseAddress *string `json:"base_address"`
	MXFound     bool    `json:"mx_found"`
	MXHost      *string `json:"mx_host"`
	Suppressed  bool    `json:"suppressed"`
	History     *string `json:"history"`

	// The cost of this validation, in credits, and how long it took.
	Credits    int `json:"credits"`
	DurationMS int `json:"duration_ms"`

	// The gates that /send runs.
	Valid  bool `json:"valid"`
	Checks []struct {
		Check  string `json:"check"`
		OK     bool   `json:"ok"`
		Detail string `json:"detail"`
	} `json:"checks"`
}

// Error is an answer that is not a success. Status is 0 for a network error. Check and
// Address come with a 422 of a check; RetryAfter and BuyURL with a 429.
type Error struct {
	Status     int    `json:"-"`
	Message    string `json:"error"`
	Check      string `json:"check"`
	Address    string `json:"address"`
	RetryAfter int    `json:"retry_after"`
	BuyURL     string `json:"buy_url"`
}

func (e *Error) Error() string {
	if e.Status == 0 {
		return "onebillion: " + e.Message
	}

	return fmt.Sprintf("onebillion: %d %s", e.Status, e.Message)
}

// NoCredits says if err is a 429: the workspace has no credits for these recipients today.
// The credits of the next day come at 00:00 UTC (Error.RetryAfter seconds).
func NoCredits(err error) bool { return status(err) == http.StatusTooManyRequests }

// Refused says if err is a 422: a bad message or a recipient that failed a check.
func Refused(err error) bool { return status(err) == http.StatusUnprocessableEntity }

func status(err error) int {
	var e *Error

	if errors.As(err, &e) {
		return e.Status
	}

	return -1
}

// SendOption changes one send.
type SendOption func(*string)

// IdempotencyKey sets the key of a send: the same key in 30 days returns the first answer.
// Without it, the client makes one, the same for its retries.
func IdempotencyKey(key string) SendOption { return func(k *string) { *k = key } }

// Send posts a mail.
func (c *Client) Send(ctx context.Context, m Mail, opts ...SendOption) (Accepted, error) {
	key := newKey()

	for _, opt := range opts {
		opt(&key)
	}

	body := struct {
		Mail
		Body map[string]string `json:"body,omitempty"`
	}{Mail: m}

	if m.Text != "" || m.HTML != "" {
		body.Body = map[string]string{}

		if m.Text != "" {
			body.Body["text"] = m.Text
		}

		if m.HTML != "" {
			body.Body["html"] = m.HTML
		}
	}

	var out Accepted

	return out, c.do(ctx, http.MethodPost, "/send", body, key, &out)
}

// Message reads one recipient.
func (c *Client) Message(ctx context.Context, id string) (Message, error) {
	var out Message

	return out, c.do(ctx, http.MethodGet, "/messages/"+url.PathEscape(id), nil, "", &out)
}

// Unsuppress takes an address off the suppression list of the app.
func (c *Client) Unsuppress(ctx context.Context, address string) error {
	return c.do(ctx, http.MethodDelete, "/suppressions/"+url.PathEscape(address), nil, "", nil)
}

// Validate returns the verdict of an address (result, reason, score and the signals). It sends
// nothing and costs 100 credits.
func (c *Client) Validate(ctx context.Context, address string) (Validation, error) {
	var out Validation

	return out, c.do(ctx, http.MethodPost, "/validate", map[string]string{"address": address}, "", &out)
}

func (c *Client) do(ctx context.Context, method, path string, in any, key string, out any) error {
	var payload []byte

	if in != nil {
		payload, _ = json.Marshal(in)
	}

	try := 1

	for {
		err := c.once(ctx, method, path, payload, key, out)

		if !retryable(err) || try == tries {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.backoff << (try - 1)):
		}

		try++
	}
}

func (c *Client) once(ctx context.Context, method, path string, payload []byte, key string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.url+path, bytes.NewReader(payload))

	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "onebillion-go/"+Version)

	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}

	res, err := c.http.Do(req)

	if err != nil {
		return &Error{Message: err.Error()}
	}

	defer res.Body.Close()

	data, _ := io.ReadAll(res.Body)

	if res.StatusCode >= 300 {
		e := &Error{Status: res.StatusCode, Message: http.StatusText(res.StatusCode)}

		_ = json.Unmarshal(data, e)

		return e
	}

	if out == nil || len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, out)
}

func retryable(err error) bool {
	var e *Error

	return errors.As(err, &e) && (e.Status == 0 || e.Status == http.StatusServiceUnavailable)
}

func newKey() string {
	b := make([]byte, 16)

	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}
