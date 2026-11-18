# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-11-18

### Added

- **`Validate` returns the verdict of an address:** `Result` (`deliverable`, `risky` or `undeliverable`), `Reason`, `Score` (from 0 to 1) and the signals (`DidYouMean`, `Free`, `Role`, `Disposable`, `Generated`, `Subaddress`, `MXFound`, `Suppressed`, `History`, ...). `Valid` and `Checks` stay as before.
- **The cost and the time of a validation:** `Credits` and `DurationMS`.

### Changed

- **A validation costs 100 credits.** Before, it was free.

## [0.1.0] - 2026-11-17

### Added

- The first release: `Send` (text, HTML, attachments, raw MIME, priority), `Message`, `Unsuppress` and `Validate`.
- A retry of a `503` and of a network error, 3 tries in all, with the same `Idempotency-Key`.
- `NoCredits` (429) and `Refused` (422), and `*Error` with `Status` and `Message`.

[Unreleased]: https://github.com/clowk-in/1billion-go/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/clowk-in/1billion-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/clowk-in/1billion-go/releases/tag/v0.1.0
