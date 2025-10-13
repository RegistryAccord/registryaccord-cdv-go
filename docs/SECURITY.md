# Security Policy

Normative terms (MUST/SHOULD/MAY) follow RFC 2119/8174 as in upstream `TERMINOLOGY.md`.

## Responsible Disclosure
- Report suspected vulnerabilities privately to security@registryaccord.org.
- Provide reproduction details, impact assessment, and affected versions.
- We aim to acknowledge within 2 business days and provide updates until resolution.

## Threat Outline and Mitigations

This document outlines the key security threats to the RegistryAccord Creator Data Vault (CDV) service and the mitigations in place.

### 1. Authentication and Authorization Threats

**Threat**: Unauthorized access to user data through forged or stolen JWTs.
**Mitigation**:
- JWTs are validated using JWKS from the identity service with Ed25519 algorithm
- Short TTL for JWTs to limit impact of stolen tokens
- DID subject validation to ensure users can only access their own data
- Secure JWKS caching with short TTL (5 minutes) to ensure fresh keys

### 2. Data Integrity Threats

**Threat**: Malicious or malformed data stored in the CDV.
**Mitigation**:
- Schema validation for all record types using JSON Schema
- Content identifiers (CIDs) for immutable record references
- Checksum verification for media uploads
- Input validation at all API boundaries

### 3. Data Confidentiality Threats

**Threat**: Exposure of sensitive user data.
**Mitigation**:
- TLS enforcement for all network communication
- No secrets, credentials, or PII logged to system logs
- S3-compatible storage with access controls
- Database access restricted to service account with minimal privileges

### 4. Denial of Service Threats

**Threat**: Resource exhaustion through malicious requests.
**Mitigation**:
- Request size limits for API endpoints
- Media size limits for uploads
- Rate limiting at infrastructure level
- Timeout configuration for HTTP handlers
- Resource limits for database connections

### 5. Schema Evolution Threats

**Threat**: Breaking changes to schemas affecting data compatibility.
**Mitigation**:
- Namespace-based schema versioning with deprecation policy
- Dynamic schema resolution from central specs repository
- Configurable policy for rejecting deprecated schemas
- Schema version tracking with all stored records

### 6. Event Streaming Threats

**Threat**: Compromised event data or unauthorized event consumption.
**Mitigation**:
- NATS JetStream with authentication and access controls
- Event deduplication to prevent duplicate processing
- Structured event envelopes with correlation IDs
- Secure event transport over TLS

## Scope
- This repository: server code, build and CI configurations, and docs.
- Excludes upstream specs; see the specs repo for its posture.

## Expectations
- NO secrets in the repository or logs. API keys, credentials, and tokens MUST NOT be committed or logged.
- Prefer least-privilege for services and tokens.
- All external input MUST be validated at boundaries.

## Process
- All PRs run `gosec` and static analysis in CI.
- Security-impacting changes SHOULD include a note in the PR and, if policy-level, an ADR under `docs/DECISIONS/`.

See upstream governance and terminology for normative usage: `GOVERNANCE.md`, `TERMINOLOGY.md`.
