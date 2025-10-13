# registryaccord-cdv-go
Go implementation of a RegistryAccord Creator Data Vault (CDV) service. This service provides secure storage and management of creator data with schema validation, JWT authentication, and event streaming.

Implements Phase 1 CDV requirements including:
- RESTful API endpoints for record and media operations
- Schema validation for supported collections
- JWT/DID authorization
- PostgreSQL storage
- NATS JetStream eventing
- S3-compatible media storage
- Structured logging and observability

License

“This project is licensed under the GNU Affero General Public License v3.0 (AGPL‑3.0). For organizations unable to comply with AGPL obligations (including network use), commercial licenses are available. Contact legal@registryaccord.org.”

## Quick start

Requirements: Go 1.25+, `make`, `golangci-lint` (optional for local checks).

For local development, copy `.env.example` to `.env` and customize as needed:

```bash
cp .env.example .env
```

Build and run:

```bash
make build
./bin/cdvd
```

Develop:

```bash
make fmt       # format
make lint      # static analysis
make test      # tests with race+coverage
make cover     # coverage summary+HTML
```

## Environment Variables

- `CDV_ENV` - Deployment environment (dev, staging, prod) (default: dev)
- `CDV_PORT` - HTTP server port (default: 8080)
- `CDV_DB_DSN` - PostgreSQL connection string
- `CDV_NATS_URL` - NATS server URL
- `CDV_S3_ENDPOINT` - S3-compatible storage endpoint
- `CDV_S3_REGION` - S3 region (default: us-east-1)
- `CDV_S3_BUCKET` - S3 bucket name
- `CDV_S3_ACCESS_KEY` - S3 access key
- `CDV_S3_SECRET_KEY` - S3 secret key
- `CDV_JWT_ISSUER` - Expected JWT issuer
- `CDV_JWT_AUDIENCE` - Expected JWT audience
- `IDENTITY_URL` - Identity service URL for DID validation
- `CDV_SPECS_URL` - URL to the specs repository for schema resolution (default: https://raw.githubusercontent.com/RegistryAccord/registryaccord-specs/main/schemas)
- `CDV_REJECT_DEPRECATED_SCHEMAS` - Whether to reject deprecated schemas (default: false)
- `CDV_CORS_ALLOWED_ORIGINS` - Comma-separated list of allowed origins for CORS (default: empty, which means deny all)

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
