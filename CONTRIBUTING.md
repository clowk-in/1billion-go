# Contributing to 1billion-go

Thank you for your help. Bug reports, docs fixes and pull requests are welcome.

## Before you start

- Search the [issues](https://github.com/clowk-in/1billion-go/issues) first.
- For a large change, open an issue and describe the idea before you write code.
- Security problems: do not open a public issue. See [SECURITY.md](SECURITY.md).
- This SDK follows the [1billion API](https://1billion.clowk.in/openapi.yml). A new endpoint comes to the 3 SDKs
  ([Go](https://github.com/clowk-in/1billion-go), [Ruby](https://github.com/clowk-in/1billion-ruby),
  [JavaScript](https://github.com/clowk-in/1billion-js)) with the same names and the same rules.

## Set up

You need Go 1.24 or newer.

```bash
git clone https://github.com/clowk-in/1billion-go && cd 1billion-go
make race
make lint
make cover
```

The unit tests use a local test server: no network and no API key. The integration test runs only when
`ONEBILLION_URL` is set:

```bash
ONEBILLION_URL=https://1billion.clowk.in ONEBILLION_KEY=<a test key> ONEBILLION_FROM="Test <hi@your-domain.com>" go test ./...
```

## Rules

- **Test first.** Write the failing test, then the code.
- **Coverage stays at 100%.** `make cover` fails below it.
- **Format.** `gofmt -l .` prints nothing, and `go vet ./...` is clean (`make lint`).
- **Blank lines.** Put one blank line before `if`, `for`, `switch`, `return`, `defer` and `go` (also before
  `if err != nil`), after a closing `}`, and between groups of lines that do different things.
- **No dependencies.** Only the standard library.
- **Simple.** Do the minimum that solves the problem. No speculative options.
- **The same rules as the other SDKs:** a retry only of `503` and network errors, 3 tries in all, with the same
  `Idempotency-Key`.
- **Docs.** A change that alters behavior updates `README.md` and the `Unreleased` section of
  [CHANGELOG.md](CHANGELOG.md) in the same pull request.

## Pull requests

1. Fork, and make a branch from `main`.
2. Make small commits. Use [Conventional Commits](https://www.conventionalcommits.org) for each message and the
   pull request title: `type(scope): summary`, with the types `feat`, `fix`, `docs`, `refactor`, `perf`, `test`,
   `build`, `ci` and `chore`. `release: X.Y.Z` is only for `make tag`.
3. Run `make race lint cover`.
4. Open the pull request and fill in the template.

## Releases

The maintainers release from `main`:

- `make tag patch|minor|major` bumps `Version` in `onebillion.go`, closes the `Unreleased` section of
  `CHANGELOG.md`, makes the commit `release: X.Y.Z` and the tag `vX.Y.Z`, and asks before it pushes.
  `make -n tag minor` shows the next version and writes nothing.
- Or merge a commit with the subject `tag(patch): ...`, `tag(minor): ...` or `tag(major): ...`. When CI
  passes on `main`, `tag.yml` runs the same script.

When CI passes on the release commit, `release.yml` makes the GitHub release with the notes of
`CHANGELOG.md`. The tag is the Go release: `go get github.com/clowk-in/1billion-go@vX.Y.Z` works at once.

## License

By contributing, you agree that your work is licensed under the [MIT license](LICENSE).
