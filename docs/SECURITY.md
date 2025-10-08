# Security Policy

Normative terms (MUST/SHOULD/MAY) follow RFC 2119/8174 as in upstream `TERMINOLOGY.md`.

## Responsible Disclosure
- Report suspected vulnerabilities privately to security@registryaccord.org.
- Provide reproduction details, impact assessment, and affected versions.
- We aim to acknowledge within 2 business days and provide updates until resolution.

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
