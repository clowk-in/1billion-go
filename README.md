# 1billion-go

The Go client of [1billion](https://1billion.clowk.in): transactional e-mail on its own sending infrastructure. Use it only on a
server: an API key must never reach a browser.

```bash
go get github.com/clowk/1billion-go
```

```go
client := onebillion.New(os.Getenv("ONEBILLION_KEY"))

out, err := client.Send(ctx, onebillion.Mail{
	From:    "App <hi@seu-dominio.com>",
	To:      []string{"jane@exemplo.com"},
	Subject: "Seu código",
	Text:    "Seu código é 482 913",
}, onebillion.IdempotencyKey("login-jane-482913"))

switch {
case onebillion.NoCredits(err): // 402: buy credits in the console
case onebillion.Refused(err):   // 422: a bad message, or a recipient that failed a check
case err != nil:                // the rest; *onebillion.Error has Status and Message
}

msg, err := client.Message(ctx, out.Messages[0].ID)
err = client.Unsuppress(ctx, "jane@exemplo.com")
v, err := client.Validate(ctx, "jane@exemplo.com")
```

- A `503` or a network error is tried again, 3 times in all, with the same `Idempotency-Key` (made for you when
  you do not pass one). No other error is tried again.
- `onebillion.WithURL`, `WithHTTPClient` and `WithBackoff` change the client.

The API is in [openapi.yml](https://1billion.clowk.in/openapi.yml).

## Tests

```bash
go test -race ./...
ONEBILLION_URL=http://localhost:3100 ONEBILLION_KEY=dev-token ONEBILLION_FROM="Dev <hi@dev.test>" go test ./...
```
