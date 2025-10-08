# registryaccord-cdv-go
Go implementation of a RegistryAccord CDV service (identity issuance and DID operations). Aligns API naming with upstream Lexicon NSIDs as a schema language only (see `registryaccord-specs/schemas/INDEX.md`).

License

“This project is licensed under the GNU Affero General Public License v3.0 (AGPL‑3.0). For organizations unable to comply with AGPL obligations (including network use), commercial licenses are available. Contact legal@registryaccord.org.”

## Quick start

Requirements: Go 1.25+, `make`, `golangci-lint` (optional for local checks).

Build and run:

```bash
make build
./bin/identityd
```

Develop:

```bash
make fmt       # format
make lint      # static analysis
make test      # tests with race+coverage
make cover     # coverage summary+HTML
```

## Documentation

- Coding standards: `docs/CODING_STANDARDS.md`
- Architecture: `docs/ARCHITECTURE.md`
- Security policy: `docs/SECURITY.md`
- AI guide: `docs/AI_GUIDE.md`
- Decisions: `docs/DECISIONS/`

Upstream specs (binding inputs):

- `registryaccord-specs/README.md`
- `registryaccord-specs/schemas/INDEX.md`
- `registryaccord-specs/schemas/SPEC-README.md`
- `registryaccord-specs/GOVERNANCE.md`
- `registryaccord-specs/TERMINOLOGY.md`
- `registryaccord-specs/CONTRIBUTING.md`

Note: Normative terms (MUST/SHOULD/MAY) follow RFC 2119/8174 as used in upstream `TERMINOLOGY.md`.

## Contributing

See `CONTRIBUTING.md`. Keep diffs minimal, update docs/ADRs when changing public behavior, and align API naming with upstream NSIDs per `schemas/INDEX.md`.

## License

AGPL-3.0; see `LICENSE`. Commercial license available; contact legal@registryaccord.org.
