package onebillion

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Tolerance is how old (or how far in the future) a webhook timestamp can be.
const Tolerance = 5 * time.Minute

// ErrInvalidSignature is a bad or missing webhook signature, or a timestamp out of the
// Tolerance. Check it with errors.Is.
var ErrInvalidSignature = errors.New("onebillion: invalid signature")

// EventMailReceived is the type of the event of a received mail: a mail to an address whose
// route is "webhook". Its Event has Data.
const EventMailReceived = "mail.received"

// Event is the body of a webhook.
type Event struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	To        string `json:"to"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"created_at"`

	// Data is the received mail of a mail.received event; nil for the other events.
	Data *MailReceived `json:"data,omitempty"`
}

// MailReceived is the mail of a mail.received event, whole. 1billion keeps no content, so this
// is the only time the app gets it. A field that the mail does not have is empty.
type MailReceived struct {
	ID          string         `json:"id"`
	From        string         `json:"from"`
	To          string         `json:"to"`
	MailFrom    string         `json:"mail_from"`
	Subject     string         `json:"subject"`
	MessageID   string         `json:"message_id"`
	Text        string         `json:"text"`
	HTML        string         `json:"html"`
	Attachments []ReceivedFile `json:"attachments"`
	Auth        MailAuth       `json:"auth"`
	Size        int            `json:"size"`
	ReceivedAt  string         `json:"received_at"`
}

// ReceivedFile is one attachment of a received mail: its name, type and size, never its content.
type ReceivedFile struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// MailAuth is what the mx of 1billion found of the sender: "pass", "fail", "none" and the others
// of RFC 8601.
type MailAuth struct {
	SPF   string `json:"spf"`
	DKIM  string `json:"dkim"`
	DMARC string `json:"dmarc"`
}

// VerifySignature checks the signature of a webhook and returns its event. payload is the raw
// body. 1B-Signature is "sha256=<hex>", with hex = HMAC-SHA256(secret, "<1B-Timestamp>.<body>").
// An empty secret reads ONEBILLION_WEBHOOK_SECRET.
func VerifySignature(payload []byte, header http.Header, secret string) (Event, error) {
	if secret == "" {
		secret = os.Getenv("ONEBILLION_WEBHOOK_SECRET")
	}

	if secret == "" {
		return Event{}, errors.New("onebillion: no webhook secret: set ONEBILLION_WEBHOOK_SECRET or pass it")
	}

	ts := lookup(header, "1B-Timestamp")
	signature := lookup(header, "1B-Signature")

	if ts == "" || signature == "" {
		return Event{}, fmt.Errorf("%w: no 1B-Timestamp or 1B-Signature header", ErrInvalidSignature)
	}

	seconds, err := strconv.ParseInt(ts, 10, 64)

	if err != nil || time.Since(time.Unix(seconds, 0)).Abs() > Tolerance {
		return Event{}, fmt.Errorf("%w: the timestamp is out of the tolerance", ErrInvalidSignature)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(payload)

	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return Event{}, fmt.Errorf("%w: the signature does not match", ErrInvalidSignature)
	}

	var event Event

	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, fmt.Errorf("%w: the body is not JSON", ErrInvalidSignature)
	}

	return event, nil
}

// lookup reads a header in any case, also from a map that was not made with Header.Set.
func lookup(header http.Header, name string) string {
	if value := header.Get(name); value != "" {
		return value
	}

	for key, values := range header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}

	return ""
}
