# Contributing to registryaccord-cdv-go

This repo implements a RegistryAccord CDV service in Go. Normative terms (MUST/SHOULD/MAY) follow RFC 2119/8174 and upstream `TERMINOLOGY.md`. We align API naming and behavior to the upstream specs repo (`registryaccord-specs`) per `schemas/INDEX.md`, `schemas/SPEC-README.md`, and `GOVERNANCE.md`.

## Workflow
- Open an issue describing the change and reference upstream specs/ADRs as applicable.
- Keep PRs minimal and focused; include rationale and links to issues/ADRs.
- If public behavior changes, you MUST update docs and ADRs.

## Local Development
- Requirements: Go 1.25+, `make`, `golangci-lint`.
- Commands:
  - `make fmt` – format
  - `make lint` – static analysis (gofumpt, govet, errcheck, staticcheck, revive, gosec)
  - `make test` – run tests with race and coverage
  - `make build` – build service in `cmd/identityd/`
  - `make cover` – coverage report

## API Alignment
- Names and shapes SHOULD reflect upstream Lexicon NSIDs as a schema language only (e.g., `com.registryaccord.identity`). See `schemas/INDEX.md` for current catalog.

## Docs & ADRs
- Update `README.md`, `docs/` (AI_GUIDE, CODING_STANDARDS, SECURITY, ARCHITECTURE), and add/change ADRs under `docs/DECISIONS/` when altering public behavior.

## Security
- Do not commit secrets. Logs MUST NOT contain secrets/PII.
- CI runs `gosec`; address high/medium findings before merge.

## DCO
- Sign your commits using `git commit -s`.

See upstream: `../registryaccord-specs/CONTRIBUTING.md`, `GOVERNANCE.md`, `TERMINOLOGY.md`.
