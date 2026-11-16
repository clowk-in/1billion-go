package onebillion

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

// server answers from a list and records each request.
type server struct {
	mu       sync.Mutex
	answers  []func(http.ResponseWriter)
	requests []*http.Request
	bodies   []map[string]any
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var body map[string]any

	data, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(data, &body)

	s.requests = append(s.requests, r)
	s.bodies = append(s.bodies, body)

	answer := s.answers[0]
	s.answers = s.answers[1:]

	answer(w)
}

func reply(status int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func start(t *testing.T, answers ...func(http.ResponseWriter)) (*server, *Client) {
	t.Helper()

	s := &server{answers: answers}
	ts := httptest.NewServer(s)

	t.Cleanup(ts.Close)

	return s, New("key", WithURL(ts.URL), WithBackoff(time.Millisecond), WithHTTPClient(ts.Client()))
}

func TestSend(t *testing.T) {
	s, c := start(t, reply(202, `{"messages":[{"id":"m1","to":"ana@x.com","status":"accepted"}]}`))

	out, err := c.Send(context.Background(), Mail{From: "App <hi@app.com>", To: []string{"ana@x.com"}, Subject: "Oi", Text: "Olá", HTML: "<p>Olá</p>"}, IdempotencyKey("k1"))

	if err != nil || out.Messages[0].ID != "m1" {
		t.Fatalf("out = %+v, err = %v", out, err)
	}

	r, body := s.requests[0], s.bodies[0]

	if r.Method != http.MethodPost || r.URL.Path != "/send" || r.Header.Get("Idempotency-Key") != "k1" || r.Header.Get("Authorization") != "Bearer key" {
		t.Fatalf("request = %s %s %v", r.Method, r.URL.Path, r.Header)
	}

	if r.Header.Get("User-Agent") != "onebillion-go/"+Version {
		t.Fatalf("user agent = %q", r.Header.Get("User-Agent"))
	}

	if b := body["body"].(map[string]any); b["text"] != "Olá" || b["html"] != "<p>Olá</p>" || body["subject"] != "Oi" {
		t.Fatalf("body = %v", body)
	}
}

func TestSendRawHasNoBody(t *testing.T) {
	s, c := start(t, reply(202, `{"messages":[]}`))

	if _, err := c.Send(context.Background(), Mail{To: []string{"a@x.com"}, Raw: "Subject: x\r\n\r\nhi"}); err != nil {
		t.Fatal(err)
	}

	if _, ok := s.bodies[0]["body"]; ok || s.bodies[0]["raw"] == nil {
		t.Fatalf("body = %v", s.bodies[0])
	}
}

func TestRetriesWithTheSameKey(t *testing.T) {
	broken := func(w http.ResponseWriter) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}

	s, c := start(t, reply(503, `{"error":"try again later"}`), broken, reply(202, `{"messages":[]}`))

	if _, err := c.Send(context.Background(), Mail{From: "a@app.com", To: []string{"b@x.com"}, Subject: "s", Text: "t"}); err != nil {
		t.Fatal(err)
	}

	if len(s.requests) != 3 {
		t.Fatalf("requests = %d", len(s.requests))
	}

	key := s.requests[0].Header.Get("Idempotency-Key")

	if key == "" || s.requests[2].Header.Get("Idempotency-Key") != key {
		t.Fatal("the retries must keep the first key")
	}
}

func TestGivesUpAfter3Tries(t *testing.T) {
	s, c := start(t, reply(503, `{}`), reply(503, `{}`), reply(503, `{}`))

	_, err := c.Message(context.Background(), "m1")

	if status(err) != 503 || len(s.requests) != 3 {
		t.Fatalf("err = %v, requests = %d", err, len(s.requests))
	}
}

func TestNeverRetriesOtherErrors(t *testing.T) {
	s, c := start(t, reply(422, `{"error":"x@tmp.com: disposable","check":"disposable","address":"x@tmp.com"}`))

	_, err := c.Send(context.Background(), Mail{From: "a@app.com", To: []string{"x@tmp.com"}, Subject: "s", Text: "t"})

	var e *Error

	if !Refused(err) || !errors.As(err, &e) || e.Check != "disposable" || e.Address != "x@tmp.com" || len(s.requests) != 1 {
		t.Fatalf("err = %v", err)
	}

	if e.Error() != "onebillion: 422 x@tmp.com: disposable" {
		t.Fatalf("message = %q", e.Error())
	}
}

func TestNoCredits(t *testing.T) {
	_, c := start(t, reply(402, `{"error":"not enough credits","buy_url":"https://1billion.clowk.in/usage"}`))

	_, err := c.Send(context.Background(), Mail{From: "a@app.com", To: []string{"b@x.com"}, Subject: "s", Text: "t"})

	var e *Error

	if !NoCredits(err) || !errors.As(err, &e) || e.BuyURL == "" || NoCredits(errors.New("x")) {
		t.Fatalf("err = %v", err)
	}
}

func TestMessageUnsuppressValidate(t *testing.T) {
	s, c := start(t, reply(200, `{"id":"m1","status":"sent"}`), reply(204, ``), reply(200, `{"valid":false,"checks":[{"check":"disposable","ok":false}]}`))
	ctx := context.Background()

	msg, err := c.Message(ctx, "m1")

	if err != nil || msg.Status != "sent" {
		t.Fatalf("msg = %+v, err = %v", msg, err)
	}

	if err := c.Unsuppress(ctx, "ana@x.com"); err != nil {
		t.Fatal(err)
	}

	v, err := c.Validate(ctx, "x@tmp.com")

	if err != nil || v.Valid || v.Checks[0].Check != "disposable" {
		t.Fatalf("v = %+v, err = %v", v, err)
	}

	if s.requests[1].URL.EscapedPath() != "/suppressions/ana@x.com" || s.bodies[2]["address"] != "x@tmp.com" {
		t.Fatalf("paths = %s", s.requests[1].URL.EscapedPath())
	}
}

func TestBadJSONAndBadURL(t *testing.T) {
	_, c := start(t, reply(200, `not json`))

	if _, err := c.Message(context.Background(), "m1"); err == nil {
		t.Fatal("bad JSON must fail")
	}

	if _, err := New("key", WithURL("://bad")).Message(context.Background(), "m1"); err == nil {
		t.Fatal("bad URL must fail")
	}
}

func TestNetworkErrorMessageAndCancel(t *testing.T) {
	c := New("key", WithURL("http://127.0.0.1:1"), WithBackoff(time.Hour))
	ctx, cancel := context.WithCancel(context.Background())

	time.AfterFunc(20*time.Millisecond, cancel)

	_, err := c.Message(ctx, "m1")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}

	e := &Error{Message: "refused"}

	if e.Error() != "onebillion: refused" {
		t.Fatalf("message = %q", e.Error())
	}
}

func TestDefaults(t *testing.T) {
	c := New("key")

	if c.url != DefaultURL || c.http.Timeout != 10*time.Second {
		t.Fatalf("client = %+v", c)
	}
}

// Against a real console: ONEBILLION_URL=http://localhost:3100 ONEBILLION_KEY=dev-token
// ONEBILLION_FROM="Dev <hi@dev.test>" go test ./...
func TestIntegration(t *testing.T) {
	base := os.Getenv("ONEBILLION_URL")

	if base == "" {
		t.Skip("set ONEBILLION_URL to run")
	}

	c := New(os.Getenv("ONEBILLION_KEY"), WithURL(base))
	ctx := context.Background()

	out, err := c.Send(ctx, Mail{From: os.Getenv("ONEBILLION_FROM"), To: []string{"sdk-go@gmail.com"}, Subject: "SDK Go", Text: "Olá"})

	if err != nil {
		t.Fatal(err)
	}

	msg, err := c.Message(ctx, out.Messages[0].ID)

	if err != nil || msg.ID != out.Messages[0].ID {
		t.Fatalf("msg = %+v, err = %v", msg, err)
	}
}
