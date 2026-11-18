.PHONY: test race lint cover tag

test:
	go test ./...

race:
	go test -race ./...

# gofmt prints nothing and go vet is clean.
lint:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...

# All code at 100%.
cover:
	go test -coverprofile=cover.out ./...
	go tool cover -func=cover.out | awk '$$3 != "100.0%" { print } END { exit ($$3 == "100.0%" ? 0 : 1) }'

# Release: make tag patch|minor|major. Bumps the version and CHANGELOG.md,
# commits "release: X.Y.Z", tags vX.Y.Z, and asks before the push. With
# make's --dry-run (-n) or DRY=1, it only shows the next version. YES=1
# pushes without asking. See scripts/release.sh.
RELEASE_DRY := $(if $(or $(DRY),$(findstring n,$(filter-out -%,$(firstword $(MAKEFLAGS))))),--dry-run)

ifneq ($(filter tag,$(firstword $(MAKECMDGOALS))),)
MAKEFLAGS += --silent
BUMP := $(word 2,$(MAKECMDGOALS))

ifneq ($(BUMP),)
$(eval $(BUMP): tag ;)
endif
endif

tag:
	+@scripts/release.sh $(or $(BUMP),patch) $(RELEASE_DRY) $(if $(YES),--yes)
