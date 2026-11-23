package onebillion

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	testSecret  = "whsec_test"
	testPayload = `{"id":"e1","type":"delivered","message_id":"m1","to":"ana@x.com","detail":"250 ok","created_at":"2026-11-18T10:00:00.000Z"}`
)

func signed(at time.Time, secret, payload string) http.Header {
	ts := strconv.FormatInt(at.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + payload))

	h := http.Header{}
	h.Set("1B-Event-Id", "e1")
	h.Set("1B-Timestamp", ts)
	h.Set("1B-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))

	return h
}

func TestVerifySignature(t *testing.T) {
	event, err := VerifySignature([]byte(testPayload), signed(time.Now(), testSecret, testPayload), testSecret)

	if err != nil || event.Type != "delivered" || event.MessageID != "m1" || event.To != "ana@x.com" {
		t.Fatalf("event = %+v, err = %v", event, err)
	}
}

// A mail.received event has the received mail in Data, with its own type. The other events
// have no Data.
func TestVerifySignatureMailReceived(t *testing.T) {
	payload := `{"id":"e2","type":"mail.received","message_id":"","to":"feedback@example.com","detail":"","created_at":"2026-11-19T12:00:01Z",` +
		`"data":{"id":"r1","from":"Ana <ana@example.org>","to":"feedback@example.com","mail_from":"ana@example.org","subject":"Oi",` +
		`"message_id":"<a@example.org>","text":"Oi, tudo bem?","html":null,` +
		`"attachments":[{"filename":"nota.pdf","content_type":"application/pdf","size":48213}],` +
		`"auth":{"spf":"pass","dkim":"pass","dmarc":"pass"},"size":52011,"received_at":"2026-11-19T12:00:00.000Z"}}`

	event, err := VerifySignature([]byte(payload), signed(time.Now(), testSecret, payload), testSecret)

	if err != nil || event.Type != EventMailReceived || event.Data == nil {
		t.Fatalf("event = %+v, err = %v", event, err)
	}

	mail := *event.Data
	file := ReceivedFile{Filename: "nota.pdf", ContentType: "application/pdf", Size: 48213}

	if mail.Subject != "Oi" || mail.Text != "Oi, tudo bem?" || mail.HTML != "" || mail.MailFrom != "ana@example.org" || mail.Size != 52011 {
		t.Fatalf("mail = %+v", mail)
	}

	if len(mail.Attachments) != 1 || mail.Attachments[0] != file || mail.Auth != (MailAuth{SPF: "pass", DKIM: "pass", DMARC: "pass"}) {
		t.Fatalf("mail = %+v", mail)
	}

	delivered, _ := VerifySignature([]byte(testPayload), signed(time.Now(), testSecret, testPayload), testSecret)

	if delivered.Data != nil {
		t.Fatalf("a delivered event has data: %+v", delivered.Data)
	}
}

func TestVerifySignatureSecretFromEnv(t *testing.T) {
	t.Setenv("ONEBILLION_WEBHOOK_SECRET", testSecret)

	_, err := VerifySignature([]byte(testPayload), signed(time.Now(), testSecret, testPayload), "")

	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("ONEBILLION_WEBHOOK_SECRET", "")

	_, err = VerifySignature([]byte(testPayload), signed(time.Now(), testSecret, testPayload), "")

	if err == nil || !strings.Contains(err.Error(), "ONEBILLION_WEBHOOK_SECRET") || errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v", err)
	}
}

// A raw map with lower case names, as a proxy can send them, still works.
func TestVerifySignatureLowerCaseHeaders(t *testing.T) {
	h := http.Header{}

	for name, values := range signed(time.Now(), testSecret, testPayload) {
		h[strings.ToLower(name)] = values
	}

	if _, err := VerifySignature([]byte(testPayload), h, testSecret); err != nil {
		t.Fatal(err)
	}
}

func TestVerifySignatureRefuses(t *testing.T) {
	now := time.Now()
	unsigned := signed(now, testSecret, testPayload)
	unsigned.Del("1B-Signature")

	badTime := signed(now, testSecret, testPayload)
	badTime.Set("1B-Timestamp", "soon")

	notJSON := signed(now, testSecret, "not json")

	cases := map[string]struct {
		payload string
		header  http.Header
	}{
		"bad secret":   {testPayload, signed(now, "other", testPayload)},
		"changed body": {strings.Replace(testPayload, "delivered", "bounced", 1), signed(now, testSecret, testPayload)},
		"no signature": {testPayload, unsigned},
		"bad time":     {testPayload, badTime},
		"old":          {testPayload, signed(now.Add(-6*time.Minute), testSecret, testPayload)},
		"future":       {testPayload, signed(now.Add(6*time.Minute), testSecret, testPayload)},
		"not json":     {"not json", notJSON},
	}

	for name, tc := range cases {
		_, err := VerifySignature([]byte(tc.payload), tc.header, testSecret)

		if !errors.Is(err, ErrInvalidSignature) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestEmailsSend(t *testing.T) {
	s, c := start(t, reply(202, `{"messages":[{"id":"m1","to":"ana@x.com","status":"accepted"}]}`))

	out, err := c.Emails.Send(context.Background(), Mail{From: "a@app.com", To: []string{"ana@x.com"}, Subject: "s", HTML: "<p>Oi</p>"}, IdempotencyKey("k1"))

	if err != nil || out.Messages[0].ID != "m1" || s.requests[0].Header.Get("Idempotency-Key") != "k1" {
		t.Fatalf("out = %+v, err = %v", out, err)
	}
}

func TestKeyAndURLFromEnv(t *testing.T) {
	t.Setenv("ONEBILLION_API_KEY", "env-key")
	t.Setenv("ONEBILLION_URL", "http://env.test")

	c := New("")

	if c.key != "env-key" || c.url != "http://env.test" {
		t.Fatalf("client = %+v", c)
	}

	if c := New("key", WithURL("http://set.test")); c.key != "key" || c.url != "http://set.test" {
		t.Fatalf("client = %+v", c)
	}
}
