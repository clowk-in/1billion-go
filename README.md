<h1 align="center">
  <a href="https://1billion.clowk.in">1billion</a>
</h1>

<p align="center">
  <b>Transactional e-mail on its own sending infrastructure.</b><br>
  The official Go SDK.
</p>

<p align="center">
  <a href="https://1billion.clowk.in"><b>Website</b></a> ·
  <a href="https://1billion.clowk.in"><b>Get an API key</b></a> ·
  <a href="https://1billion.clowk.in/openapi.yml"><b>API reference</b></a> ·
  <a href="CHANGELOG.md"><b>Changelog</b></a>
</p>

<p align="center">
  <a href="https://github.com/clowk-in/1billion-go/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/clowk-in/1billion-go/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/clowk-in/1billion-go"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/clowk-in/1billion-go.svg"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
</p>

The official Go client of [1billion](https://1billion.clowk.in): transactional e-mail on its own sending
infrastructure. Send a login code, a receipt or a reset link with one call, and read what happened to each
recipient.

- **No dependencies.** Only the Go standard library.
- **Safe retries.** A `503` or a network error is tried again with the same `Idempotency-Key`, so a retry never
  sends a mail twice.
- **Clear errors.** `NoCredits` and `Refused` tell you what to do next.
- **Address validation.** A verdict, a score and the signals of an address, before you send to it.

> Use it only on a server. An API key must never reach a browser.

## Install

```bash
go get github.com/clowk-in/1billion-go
```

Go 1.24 or newer. The package name is `onebillion` (a Go name can not start with a digit).

## Quick start

Make an API key in the [console](https://1billion.clowk.in), and verify the domain that you send from. Put the
key in the environment:

```bash
export ONEBILLION_API_KEY=<your API key>
```

```go
package main

import (
	"context"
	"log"

	onebillion "github.com/clowk-in/1billion-go"
)

func main() {
	client := onebillion.New("") // the key of ONEBILLION_API_KEY

	out, err := client.Emails.Send(context.Background(), onebillion.Mail{
		From:    "Acme <hi@acme.com>",
		To:      []string{"jane@example.com"},
		Subject: "Your code",
		Text:    "Your code is 482 913",
	})

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("accepted: %s", out.Messages[0].ID)
}
```

`Send` returns when 1billion has the mail (`202`, status `accepted`). The delivery runs after that, with its
own retries. Each recipient gets a message id of its own.

`client.Emails.Send` reads as `resend.Emails.Send`, so a move from Resend changes few lines. It is the same
call as `client.Send`. An empty key reads `ONEBILLION_API_KEY`; the URL is `ONEBILLION_URL`, else
`https://1billion.clowk.in`.

## Verify a webhook

```go
http.HandleFunc("/webhooks/1billion", func(w http.ResponseWriter, r *http.Request) {
	payload, _ := io.ReadAll(r.Body)

	event, err := onebillion.VerifySignature(payload, r.Header, "")

	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)

		return
	}

	if event.Type == "bounced" {
		markBounced(event.To)
	}

	w.WriteHeader(http.StatusOK)
})
```

`VerifySignature` checks `1B-Signature` against the raw body and `1B-Timestamp`, and returns the `Event`
(`ID`, `Type`, `MessageID`, `To`, `Detail`, `CreatedAt`). An empty secret reads `ONEBILLION_WEBHOOK_SECRET`.
A bad or missing signature, or a timestamp more than 5 minutes old, gives an error that `errors.Is` matches to
`onebillion.ErrInvalidSignature`.

## Send

```go
out, err := client.Send(ctx, onebillion.Mail{
	From:     "Acme <hi@acme.com>",
	To:       []string{"jane@example.com"},
	Cc:       []string{"team@acme.com"},
	Bcc:      []string{"audit@acme.com"},
	ReplyTo:  "support@acme.com",
	Subject:  "Your receipt",
	Text:     "Thanks for your order.",
	HTML:     "<p>Thanks for your <b>order</b>.</p>",
	Priority: "high",
	Attachments: []onebillion.Attachment{
		{Filename: "receipt.pdf", ContentType: "application/pdf", Content: base64PDF},
	},
}, onebillion.IdempotencyKey("receipt-8812"))
```

| Field | What |
|---|---|
| `From` | a sender on a verified domain of the app, with a name or without one |
| `To`, `Cc`, `Bcc` | the recipients |
| `Text`, `HTML` | the body: one of them or both |
| `Attachments` | files, with `Content` in base64 |
| `Priority` | `normal` (the default) or `high`: mail that a person waits for (a code, a reset link) goes first |
| `Raw` | a finished MIME message, in place of the fields above |

### Idempotency

The same `IdempotencyKey` in 30 days returns the first answer and sends nothing again. Use a key that names
the mail (`receipt-8812`, `login-jane-482913`). When you do not pass a key, the client makes one and uses it
for its own retries.

## Errors

```go
_, err := client.Send(ctx, mail)

var e *onebillion.Error

errors.As(err, &e)

switch {
case onebillion.NoCredits(err):
	// 429: no credits for these recipients today. They come back at 00:00 in the
	// time zone of the workspace (e.RetryAfter seconds), or buy a pack (e.BuyURL).
case onebillion.Refused(err):
	// 422: a bad message, a domain that is not verified, or a recipient that
	// failed a check (e.Check, e.Address).
case err != nil:
	// Any other answer: e.Status and e.Message. Status is 0 for a network error.
}
```

| Status | What |
|---|---|
| `401` | a bad API key |
| `403` | the sends of the workspace are paused (low reputation); a person reviews it |
| `422` | `Refused`: fix the message or the recipient |
| `429` | `NoCredits` |
| `503` | tried again by the client |

Only a `503` and a network error are tried again: 3 tries in all, with a wait that doubles (500 ms, then 1 s).
No other error is tried again, because a second try gives the same answer.

## Read a message

```go
msg, err := client.Message(ctx, out.Messages[0].ID)

// msg.Status: accepted, queued, sending, sent, bounced or suppressed.
// msg.Attempts, msg.LastError, msg.CreatedAt, msg.UpdatedAt.
```

`sent` means that the mail server of the recipient accepted the message. Messages stay for 30 days.

## Validate an address

```go
v, err := client.Validate(ctx, "jane@gmial.com")

// v.Result:     deliverable, risky or undeliverable
// v.Reason:     why, for example "possible_typo"
// v.Score:      from 0 to 1
// v.DidYouMean: "jane@gmail.com"
// v.Disposable, v.Role, v.Free, v.Generated, v.MXFound, v.Suppressed, ...
```

A validation sends nothing and costs 100 credits.

## Suppressions

A hard bounce or a complaint puts the address on the suppression list of the app, and 1billion stops sending
to it. When the address works again:

```go
err := client.Unsuppress(ctx, "jane@example.com")
```

## Options

```go
client := onebillion.New("", // "" reads ONEBILLION_API_KEY
	onebillion.WithURL("http://localhost:3100"),
	onebillion.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}),
	onebillion.WithBackoff(time.Second),
)
```

| Option | Default | What |
|---|---|---|
| `WithURL` | `ONEBILLION_URL`, else `https://1billion.clowk.in` | another console, for example a dev stack |
| `WithHTTPClient` | a client with a 10 s timeout | your own `*http.Client` |
| `WithBackoff` | 500 ms | the first wait between tries; it doubles each time |

Each request has the `User-Agent` `onebillion-go/<version>`, so the console shows which mails came from the SDK.

## Start sending

1. Make a free account on **[1billion.clowk.in](https://1billion.clowk.in)**: 100 mails a day for free, every day, and your first domain for free.
2. Add your domain. The console gives you the DNS records, and checks them for you.
3. Make an API key, then `go get github.com/clowk-in/1billion-go`.

Your first mail goes out in a few minutes. A question about the SDK? [Open an issue](https://github.com/clowk-in/1billion-go/issues).

## Docs

- API reference: [openapi.yml](https://1billion.clowk.in/openapi.yml)
- Package docs: [pkg.go.dev](https://pkg.go.dev/github.com/clowk-in/1billion-go)
- Changes: [CHANGELOG.md](CHANGELOG.md)
- Other SDKs: [Ruby](https://github.com/clowk-in/1billion-ruby) (with ActionMailer) and
  [JavaScript](https://github.com/clowk-in/1billion-js) (Node.js, Next.js, Bun)

## Contributing

Bug reports and pull requests are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) first. For a security
problem, see [SECURITY.md](SECURITY.md). This project follows a [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE)
