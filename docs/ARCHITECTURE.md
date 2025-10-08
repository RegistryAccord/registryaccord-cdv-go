# Architecture Overview

Normative terms (MUST/SHOULD/MAY) follow RFC 2119/8174 and upstream `TERMINOLOGY.md`. API naming SHOULD align with upstream Lexicon NSIDs as a schema language only (see `schemas/INDEX.md`).

## Purpose
`registryaccord-cdv-go` provides core CDV service capabilities for identity issuance and DID operations aligned with the RegistryAccord protocol.

## Boundaries
- **Identity issuance**: create/resolve identities; map to upstream `com.registryaccord.identity` concepts.
- **DID ops**: key/material management, DID document updates (method-specific adapters kept internal).
- **Out of scope**: client SDKs, UI, and non-core experimental endpoints.

## Components
- `cmd/identityd/`: service entrypoint and wiring.
- `internal/`: implementation details (storage, DID adapters, handlers).
- `pkg/`: stable helper packages intended for reuse.
- `api/`: OpenAPI and request/response shapes that mirror upstream schema terms.

## API Surface
- HTTP JSON endpoints for identity creation and lookup; names SHOULD reflect upstream NSIDs (e.g., `/x/com.registryaccord.identity/create`).
- OpenAPI: TODO stub (`api/openapi.yaml`).

## Storage
- Pluggable storage (memory/sql/kv). Minimal initial implementation with clear interfaces in `internal/storage`.

## Integrations
- **Gateway/CDV**: Interacts per protocol to publish and resolve identity records. Align field names and statuses with upstream examples.

## Observability
- `log/slog` structured logs; metrics hooks MAY be added behind interfaces.

References: `../registryaccord-specs/README.md`, `schemas/SPEC-README.md`, `GOVERNANCE.md`, `TERMINOLOGY.md`.
