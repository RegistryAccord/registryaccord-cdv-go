# AI Guide for registryaccord-cdv-go

This guide is a prompt-ready reference for AIs and contributors. Normative terms (MUST/SHOULD/MAY) follow RFC 2119/8174 and the upstream `TERMINOLOGY.md`. Align behavior and naming with the upstream specs repo: `../registryaccord-specs/README.md`, `schemas/INDEX.md`, `schemas/SPEC-README.md`, and `GOVERNANCE.md`.

## Scope and Style
- **[language]** Go 1.25+ service, server code under `cmd/identityd/`, libraries in `internal/` and `pkg/`.
- **[formatting]** MUST run `gofumpt` (via `golangci-lint`).
- **[lint]** MUST pass `golangci-lint` with enabled linters in `.golangci.yml`.
- **[errors]** Use wrapped errors (`fmt.Errorf("...: %w", err)`), return context-aware errors to handlers.
- **[context]** Functions that do I/O MUST accept `context.Context` and honor cancellation/timeouts.
- **[logging]** Use `log/slog` with structured fields; DO NOT log secrets, PII, or credentials.
- **[api naming]** Endpoint and type names SHOULD mirror upstream Lexicon NSIDs as a schema language only (e.g., `com.registryaccord.identity` maps to our identity handlers).

## Input Validation and Error Handling
- **[validate]** Validate all external inputs. Prefer small, explicit structs and `net/mail`, `net/url`, and `time` parsing where applicable.
- **[boundaries]** Check length limits, allowlists, and enums for request fields; reject ambiguous time zones.
- **[HTTP]** Map domain errors to appropriate status codes. Include stable machine fields in JSON responses; avoid leaking internals.

## Local Dev: Run, Lint, Test
- **[prereqs]** Go 1.25+, `golangci-lint`, `pre-commit` (optional), `make`.
- **[commands]**
  - `make fmt` – apply formatting
  - `make lint` – static analysis (gofumpt, govet, errcheck, staticcheck, revive, gosec)
  - `make test` – run unit tests with race and coverage
  - `make build` – build `cmd/identityd`
  - `make cover` – generate coverage report
  - `make api-validate` – OPTIONAL: validate API naming against specs (stub)

## Minimal Diffs Policy
- **[rules]**
  - Keep PRs focused; avoid unrelated formatting churn (use `make fmt` separately).
  - Update docs alongside code when behavior changes: `README.md`, `docs/`, and ADRs under `docs/DECISIONS/`.
  - Reference upstream normative sources (TERMINOLOGY, GOVERNANCE, SPEC-README) in comments when imposing MUST/SHOULD.

## Keeping Specs in Sync
- **[schemas]** Follow `schemas/INDEX.md` for canonical NSIDs. Treat Lexicon as the schema language only (no generator coupling yet).
- **[examples]** When adding/altering public API, add or update examples under `docs/examples/` (if present) and cite which upstream schema line(s) informed the change.

## PR Checklist (quick)
- `make fmt && make lint && make test`
- If public behavior changed: update `README.md`, `docs/*`, ADRs
- If API shape touched: confirm naming aligns with `schemas/INDEX.md`
- No secrets or PII in logs; security checks pass (`gosec`)
- Minimal, well-explained diffs with links to issues/ADRs
