# 2026-05-19 - Job Descriptor Trust Boundaries

## Context

ISSUE-014 introduced `JobDescriptor` as the internal contract between ingress, feature extraction, policy, and routing. The descriptor is built from authenticated tenant context, request headers, OpenAI-compatible request fields, and client metadata.

## What I Learned

Client metadata and routing headers need to be preserved as hints without becoming policy truth. Authenticated tenant context should populate trusted tenant/project fields; header or metadata tenant/project values should only be hint fields when no authenticated context exists. Client-supplied risk, task, and sensitivity values are useful routing signals, but they must not lower the descriptor's conservative defaults before policy and classifier rules run.

Safe descriptor logging should project derived fields and counts instead of serializing raw metadata. Even metadata keys can reveal secret-like intent, so log payloads should avoid both metadata values and key names unless a future allowlist explicitly marks them safe.

## Reuse Rules

- Build descriptors from trusted auth context first, then untrusted header and metadata hints.
- Keep client risk/task/sensitivity hints separate from authoritative descriptor fields.
- Clone nested metadata before storing it on a descriptor.
- Keep prompt and message text out of descriptor structs and logging projections.
- Log metadata presence or counts, not raw metadata values or unreviewed key names.

## Failure Signals

- A request with authenticated tenant context can override `TenantID` through `metadata.tenant_id` or `X-Router-Tenant-Id`.
- `risk_level: low` from client metadata lowers a descriptor before policy/risk rules run.
- A safe log payload contains message content, metadata secrets, metadata key names such as `api_key`, or raw prompt text.
- Mutating request metadata after descriptor construction changes descriptor metadata.

## Next Checklist

- [ ] Before adding classifier writes to `JobDescriptor`, preserve the distinction between client hints and classified/policy truth.
- [ ] When adding new metadata-derived signals, decide whether they are safe to log before including them in projections.
- [ ] Add focused tests whenever new descriptor fields consume headers or metadata.
