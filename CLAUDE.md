# 1billion-go — guide for AI agents

Read this file first. It tells you what this repo is, how to test it, and its rules.

## What it is

The official Go client of [1billion](https://1billion.clowk.in), a transactional e-mail API. One package,
`onebillion`, in `onebillion.go` (the client) and `webhook.go` (`VerifySignature`, `Event`), with no
dependencies. The API it follows is
[openapi.yml](https://1billion.clowk.in/openapi.yml).

| Call | Endpoint |
|---|---|
| `Client.Send`, `Client.Emails.Send` | `POST /send` |
| `Client.Message` | `GET /messages/{id}` |
| `Client.Unsuppress` | `DELETE /suppressions/{address}` |
| `Client.Validate` | `POST /validate` |

The same SDK exists for Ruby (`clowk-in/1billion-ruby`) and JavaScript (`clowk-in/1billion-js`). The 3 keep
the same names and the same rules. A change to one usually needs the same change in the other two.

## Run and test

```bash
make race    # unit tests, with a local test server: no network
make lint    # gofmt -l . prints nothing; go vet ./... is clean
make cover   # all code at 100%
```

The integration test runs only with `ONEBILLION_URL`, `ONEBILLION_API_KEY` and `ONEBILLION_FROM`.

## Rules

- **Test first.** Write the failing test, then the code. Coverage stays at 100%.
- **The retry rule:** only a `503` and a network error are tried again, 3 tries in all, with the same
  `Idempotency-Key`. The client makes the key when the caller does not pass one.
- **No dependencies.** Only the standard library.
- **Simple:** the minimum that solves the problem. No speculative options.
- **Blank lines:** one blank line before `if`, `for`, `switch`, `return`, `defer`, `go` (also before
  `if err != nil`), after a closing `}`, and between groups of lines that do different jobs.
- **Docs change with behavior** in the same commit: `README.md` and the `Unreleased` section of `CHANGELOG.md`.
- **Docs style:** short sentences and simple words (ASD-STE100 style), as in the existing pages.
- This is a public repo. Never put an API key, a customer address or a private host in code, tests or docs.

## Release

- `make tag patch|minor|major` (`scripts/release.sh`) bumps `Version` in `onebillion.go`, closes the
  `Unreleased` section of `CHANGELOG.md`, commits `release: X.Y.Z`, tags `vX.Y.Z` and asks before the push.
  `YES=1` pushes without asking. `make -n tag minor` is a dry run.
- A commit `tag(patch|minor|major): ...` on `main` does the same through `tag.yml`, after CI passes.
- CI tests the release commit, then `release.yml` makes the GitHub release from `CHANGELOG.md`.
- Never push a `v*` tag by hand: the version in `onebillion.go` must match the tag.

## Git

- **Conventional Commits** for every commit and PR title: `type(scope): summary`, lower case type, summary in
  the imperative, no period at the end. Types: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`,
  `ci`, `chore`.
- Commit only when the maintainer asks. Do not watch CI: the maintainer follows it.
- Never credit Claude in a commit or a PR: no `Co-Authored-By: Claude` trailer, no "Generated with Claude Code"
  line. The author is always the maintainer.
