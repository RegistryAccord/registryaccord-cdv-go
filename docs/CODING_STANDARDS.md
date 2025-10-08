# Go Coding Standards

Normative terms (MUST/SHOULD/MAY) follow RFC 2119/8174 and upstream `TERMINOLOGY.md`. Align naming with upstream Lexicon NSIDs (schema language only) per `schemas/INDEX.md` and `schemas/SPEC-README.md`.

## Formatting & Linting
- **gofumpt**: Code MUST be formatted (enforced via `golangci-lint`).
- **golangci-lint**: MUST pass with configured linters: gofumpt, govet, errcheck, staticcheck, revive, gosec.
- **Imports**: Group stdlib, third-party, then local modules.

## Packages & Naming
- **Packages**: Lowercase, no underscores; short, domain-focused names in `internal/` and `pkg/`.
- **APIs**: Public handlers/types SHOULD reflect upstream NSID terms where applicable (e.g., identity operations = `com.registryaccord.identity`).

## Errors
- **Wrapping**: Wrap with `%w` (`fmt.Errorf("context: %w", err)`).
- **Sentinel**: Prefer `errors.Is/As` for checks.
- **Messages**: Lowercase, no trailing punctuation; include actionable context.

## Context
- **context.Context**: Any I/O or RPC-facing function MUST accept a `context.Context` as first param and honor deadlines/cancel.

## Logging
- **slog**: Use `log/slog` with structured fields. Include stable identifiers; DO NOT log secrets, access tokens, or PII.
- **Levels**: debug for dev detail, info for state changes, warn for recoverable anomalies, error for failures with err field.

## Testing
- **Layout**: Tests close to code; table-driven tests where practical.
- **Golden files**: Store under `testdata/`; verify deterministically.
- **Race & coverage**: CI runs with `-race` and coverage thresholds tracked.

## Security
- **gosec**: MUST pass in CI; address high/medium findings.
- **Input validation**: Validate all external inputs at boundaries; enforce length and type constraints.
- **Dependencies**: Keep modules up to date; prefer minimal, vetted deps.

## References
- Upstream terminology and normative usage: `TERMINOLOGY.md`.
- Authoring and evolution rules: `schemas/SPEC-README.md`, `GOVERNANCE.md`.
