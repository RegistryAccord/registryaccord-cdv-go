# CDV Requirements

## Purpose
- Define Phase 1 requirements for the Creator Data Vault (CDV) service across APIs, schema enforcement, storage, eventing, security, observability, configuration, CI, and acceptance criteria to interoperate end‑to‑end.
- Align implementation with the specs repository (lexicons, governance, federation draft), the technology strategy (dual‑vault, event‑driven), and Phase 1 execution milestones.

## Scope
- Deliver a cloud‑first CDV with self‑host parity focused on record create and list, media upload/finalize with integrity checks, schema‑enforced writes, and event emission for records and media.
- Provide deterministic pagination and shared error taxonomy, DID/JWT authorization flows compatible with identity, and operational readiness via health, metrics, and logs.

## APIs
- POST /v1/repo/record: Accepts { collection, did, record, createdAt?, idempotencyKey? }, validates record against the stable lexicon schema for the collection, enforces author DID, persists the write, emits a RA_RECORDS event, and returns { data: { uri, cid, indexedAt } }.
- GET /v1/repo/listRecords: Supports query did, collection, limit (default 25, max 100), cursor, since?, until?, returns { data: { records: [{ uri, cid, value, indexedAt }…], nextCursor? } } with stable reverse‑chronological ordering by indexedAt.
- POST /v1/media/uploadInit: Accepts { did, mimeType, size, sha256?, filename? }, returns { data: { assetId, uploadUrl, expiresAt } }, reserves a metadata row, and enforces size and type limits before issuing a presigned URL.
- POST /v1/media/finalize: Accepts { assetId, sha256 }, verifies object presence and checksum, persists metadata, emits RA_MEDIA event, and returns { data: { assetId, uri, mimeType, size, checksum, createdAt } }.
- GET /v1/media/:assetId/meta: Returns stored media metadata without public object access, with signed URLs used internally only in Phase 1 flows.

## Auth and identity
- Use Bearer JWTs with sub set to the author DID, aud set to the CDV service audience, exp short‑lived, and signatures verified against the identity service’s issuer keys.
- Enforce author=subject for Phase 1 and document that delegated write authorization may be introduced later with explicit policy and verification changes.
- Implement JWKS discovery and caching with a short TTL, pin supported algorithms (Ed25519 in Phase 1), and fail closed on issuer or key mismatch to maintain security.

## Schema enforcement
- Enforce upstream lexicon schemas on write for profile, post, follow, like, comment, repost, moderationFlag, and mediaAsset collections, rejecting invalid records with deterministic error codes.
- Persist the exact schema version used for validation with each stored record to support audit, replay, and compatibility analysis across upgrades.
- Define behavior for deprecated schema versions as “accept with warning until sunset date” or “reject after deprecation window,” and track policy in governance notes.

## Namespace and version resolution
- Resolve collection NSIDs deterministically to a specific schema version by consulting the specs repository’s index and governance rules at service boot and on tagged updates.
- Refuse ambiguous or unknown collection identifiers with a clear validation error and log the unresolved NSID alongside correlation information.

## Storage model
- Postgres tables: accounts(did pk, created_at), records(id pk, did fk, collection, rkey, uri, cid, value jsonb, indexed_at, schema_version), media_assets(asset_id pk, did fk, uri, mime_type, size, checksum, created_at), op_log(seq pk, type, ref, did, payload jsonb, occurred_at).
- Indexes: composite indexes on (did, collection, indexed_at desc) and unique constraints for (did, collection, rkey) to ensure stable dereference and ordering.
- Media storage: S3‑compatible bucket with versioning, lifecycle, server‑side encryption, and public access disabled using path s3://ra-media/{env}/{did}/{assetId}/{filename}.

## RKey, URI, and dereference
- Generate rkey per record using ULID or a collision‑resistant scheme, ensuring uniqueness within (did, collection) under concurrent writes.
- Compose uri as did/collection/rkey and return alongside cid so clients can dereference consistently and perform optimistic concurrency checks.

## Media finalize constraints
- Enforce default max size and accepted mime types at uploadInit, allow overrides via environment with documented bounds, and reject oversize or disallowed types deterministically.
- On finalize, verify ETag or Content‑MD5 matches the provided SHA‑256 and persist the measured object size to ensure downstream data integrity.

## Eventing
- NATS JetStream streams: RA_RECORDS on subjects cdv.records.<collection>.created and RA_MEDIA on cdv.media.finalized with durable consumers for gateway fan‑out and devtools replay.
- Envelope: { type, version, occurredAt, correlationId, payload }, where payload references schema version and primary identifiers for deterministic downstream processing.
- Delivery: at‑least‑once with dedup keyed by correlationId over a two‑minute window, explicit retention limits, ack waits, and max in‑flight tuned for reliability and backpressure.

## Pagination and filtering
- Use cursor‑based pagination with stable sort by indexedAt desc and encode cursors as base64 of { lastIndexedAt, lastRkey } so page traversal remains deterministic under concurrent writes.
- Provide filters did, collection, since, until without violating sort stability and return a specific error for invalid cursors with guidance to restart pagination.

## Idempotency
- Persist idempotencyKey with a hash of the normalized request body for POST /v1/repo/record so replays within a 24‑hour window return the original response consistently.
- Document client retry guidance by error class, including backoff for rate limiting and transient infrastructure failures based on the shared taxonomy.

## Error taxonomy and envelopes
- Success envelope: { data, meta? } unified across endpoints with RFC 3339 timestamps.
- Error envelope: { error: { code, message, details?, correlationId } } using CDV_* codes mapped to HTTP statuses such as 400, 401/403, 404, 409, 429, 500, and 503.

## Observability
- Emit structured JSON logs including level, correlationId, did when authorized, route, and latency for reproducible debugging and incident analysis.
- Provide Prometheus metrics for request latency histograms, request and error counters by route and code, event publish counters, and useful buckets around 50/100/250/500 ms.
- Include OpenTelemetry spans for HTTP handlers, Postgres queries, S3 operations, and NATS publish with a default sampling ratio suitable for devstack and staging.

## Security and privacy
- Enforce TLS 1.3, hardened headers, deny‑all CORS by default, and minimal development relaxations only in devstack with explicit configuration.
- Validate schema and JWT at the write path, never log secrets or tokens, and redact or hash identifiers where appropriate while preserving diagnosability.
- Use KMS‑backed credentials in production for S3, NATS, and Postgres, forbidding plaintext secrets beyond devstack defaults.

## Configuration
- Required environment variables include CDV_DB_DSN, CDV_S3_ENDPOINT, CDV_S3_BUCKET, CDV_S3_ACCESS_KEY, CDV_S3_SECRET_KEY, CDV_NATS_URL, CDV_NATS_CREDS, CDV_JWT_ISSUER, CDV_JWT_AUDIENCE, CDV_ENV, and PORT with process environment authoritative and .env allowed only for local development.
- Expose /healthz for liveness and /readyz for readiness probes for orchestrators and local compose health checks.

## Performance and limits
- Target p95 ≤ 200 ms for listRecords and ≤ 300 ms for record create and media finalize under nominal local loads with warm caches to deliver responsive demo and devstack experiences.
- Document enforceable defaults and environment overrides for media size and mime types alongside pagination limits and request timeouts.

## Testing and conformance
- Unit tests must cover schema validation positive and negative cases, authorship checks, idempotency, cursor pagination, finalize checksum, and event publication with deterministic error bodies.
- Integration tests must exercise Postgres, NATS, and S3‑compatible storage via local compose, including chaos and retry paths to validate idempotency and dedup behavior.
- Integrate the conformance harness and the specs repo’s conformance manifest to achieve ≥95% pass rate on staging with documented deviations and remediation plans.

## CI/CD
- GitHub Actions must run fmt, golangci‑lint, unit tests with race and coverage, integration tests against the local stack, govulncheck and secret scanning, and container build and push to GHCR with SBOM and provenance.
- Protect main with required checks, CODEOWNERS reviews, signed commits, and semver‑tagged releases attaching image digests and machine‑readable API descriptions.

## Docs
- Provide README quickstart with devstack compose, API reference examples using shared envelopes and codes, ARCHITECTURE diagrams for storage and eventing, SECURITY threat outline with mitigations, and DECISIONS ADRs for schema enforcement and media finalize.
- Publish OpenAPI or equivalent machine‑readable API definitions with every tag to enable SDK/CLI generation and client validation across environments.

## Devstack integration
- Ensure a public devstack repository orchestrates identity, CDV, gateway stubs, Postgres, NATS, and MinIO with a .env.example and make targets for up, down, logs, and seed.
- Define the CDV service name, port mappings, health checks, and a deterministic seed script contract so contributors can boot and demo end‑to‑end in one command.

## Federation readiness
- Verify listRecords contracts, pagination determinism, and event payloads are sufficient for a gateway indexer to build basic feed and search without service‑specific workarounds.
- Document non‑goals and versioning notes for v0 federation so downstream consumers can target stable behavior during Phase 1.

## Acceptance criteria
- End‑to‑end flow succeeds: identity:create → session:login → post:create with optional media upload/finalize → listRecords returns expected results with deterministic pagination and envelopes.
- Reliability is demonstrated: events are published on writes, replays produce no duplicates at durable consumers, checksum is verified on finalize, and health endpoints remain green during the happy path.
- Quality and interop are proven: ≥80% coverage on core paths, green CI, logs/metrics/traces present, schemas enforced at write with deterministic errors, and conformance pass ≥95% on staging.
