# Architecture Overview

Normative terms (MUST/SHOULD/MAY) follow RFC 2119/8174 and upstream `TERMINOLOGY.md`. API naming SHOULD align with upstream Lexicon NSIDs as a schema language only (see `schemas/INDEX.md`).

## Purpose
`registryaccord-cdv-go` provides core CDV (Creator Data Vault) service capabilities for storing and managing user-generated content, media assets, and related metadata aligned with the RegistryAccord protocol.

## Boundaries
- **Record Management**: Create, store, and retrieve user-generated content records (posts, profiles, follows, etc.)
- **Media Storage**: Handle media asset uploads with checksum verification and metadata management
- **Schema Validation**: Enforce upstream lexicon schemas at write time for all supported collections
- **Event Streaming**: Publish record and media events to NATS JetStream for real-time updates
- **Authentication**: JWT-based authentication with DID validation
- **Out of scope**: Identity creation/management (handled by separate identity service), client SDKs, UI

## Components
- `cmd/cdvd/`: Service entrypoint and wiring
- `internal/model/`: Core data structures for accounts, records, and media assets
- `internal/storage/`: Storage implementations (in-memory and PostgreSQL)
- `internal/server/`: HTTP handlers and routing with JWT middleware
- `internal/schema/`: JSON schema validation for record validation
- `internal/event/`: NATS JetStream event publishing
- `internal/media/`: S3-compatible media storage operations
- `internal/identity/`: Client for interacting with the identity service

## API Surface
- RESTful HTTP JSON endpoints for record and media operations
- Standard error taxonomy with deterministic error codes
- Cursor-based pagination for list operations
- JWT-based authentication for mutating operations

## Storage
- PostgreSQL implementation for production use with schema-defined tables and indexes
- In-memory implementation for development and testing
- Tables for accounts, records, media assets, and operation logs

## Integrations
- **Identity Service**: Validate DIDs and JWT signatures via HTTP calls
- **S3 Storage**: Generate presigned URLs for direct client uploads
- **NATS JetStream**: Stream record and media events for real-time updates

## Observability
- Structured JSON logging with `log/slog`
- Correlation IDs for request tracing
- Metrics hooks for request latency and error tracking

References: `../registryaccord-specs/README.md`, `schemas/SPEC-README.md`, `GOVERNANCE.md`, `TERMINOLOGY.md`.
