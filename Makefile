# Wine Compatibility Probe: agent-friendly entry points.
# Every target is a plain command an agent can run and read.

DIST    := dist
RESULTS := results
DOCS    := docs
PROBE   := $(DIST)/probe.exe
COMPARE := $(DIST)/compare
TITLE   := Wine Compatibility Probe: Windows vs Wine vs Proton

.PHONY: all vet test build build-compare compare compare-fix e2e e2e-publish hooks secrets-scan clean

all: vet test build build-compare

## vet: static checks for the host target and the Windows target.
vet:
	go vet ./...
	GOOS=windows GOARCH=amd64 go vet -unsafeptr=false ./...

## test: unit tests (schema + compare); run on any OS.
test:
	go test ./...

## build: cross-compile the Windows probe (no C toolchain needed).
build:
	mkdir -p $(DIST)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(PROBE) ./cmd/probe
	@ls -l $(PROBE)

## build-compare: native build of the report generator.
build-compare:
	mkdir -p $(DIST)
	go build -trimpath -o $(COMPARE) ./cmd/compare

## compare: Windows vs Wine vs Proton report from results/*.json.
compare:
	go run ./cmd/compare -allow-missing -title "$(TITLE)" -out $(DOCS)/COMPARISON-REPORT.md \
		windows=$(RESULTS)/windows.json wine=$(RESULTS)/wine.json proton=$(RESULTS)/proton.json

## compare-fix: before/after report for the version-detection workaround.
compare-fix:
	go run ./cmd/compare -allow-missing -title "Fix: version detection, before vs after" -out $(DOCS)/FIX-BEFORE-AFTER.md \
		windows-before=$(RESULTS)/windows-before.json windows-after=$(RESULTS)/windows.json \
		wine-before=$(RESULTS)/wine-before.json wine-after=$(RESULTS)/wine.json \
		proton-before=$(RESULTS)/proton-before.json proton-after=$(RESULTS)/proton.json

## e2e: run the full build/test/compare pipeline on GitHub Actions and wait.
e2e:
	sh scripts/e2e.sh

## e2e-publish: same as e2e, and commit results + reports back to main.
e2e-publish:
	PUBLISH=true sh scripts/e2e.sh

## hooks: install the pre-commit secret check.
hooks:
	sh scripts/setup-hooks.sh

## secrets-scan: scan the whole tree with gitleaks (if installed).
secrets-scan:
	gitleaks detect --source . --redact --no-banner

clean:
	rm -rf $(DIST)
