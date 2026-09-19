# Security policy

Sluss is a security product: a self-hosted gateway that decides where LLM
traffic is *allowed* to go and records the evidence. We take reports seriously
and prefer coordinated disclosure.

## Reporting a vulnerability

- **Do not open a public issue** for anything that could be exploited.
- Email **security@sluss.eu** with a description, reproduction steps and the
  affected version/commit. Encrypting with our public key is welcome but not
  required.
- You will get an acknowledgement within **3 business days** and a remediation
  plan or status within **14 days**. We credit reporters in the release notes
  unless you prefer otherwise.

## Scope

In scope: the gateway (`cmd/router`), the admin console and its auth, the
policy engine (bypasses of fail-closed behaviour are high severity), the
classifier (misses that let sensitive data reach a non-compliant provider),
the audit chain and verifier (tamper-evidence), the MCP surface, and key
handling.

Out of scope: vulnerabilities in third-party LLM providers, denial of service
against your own instance, and findings that require an already-compromised
admin credential.

## What "secure by default" means here

- Secrets are referenced by **environment-variable name only**; nothing in the
  repo or the database stores a provider key or password in plain text.
- Policy is **fail-closed**: if no provider satisfies the required compliance
  tags the request is blocked with an audited 403 — never a silent fallback.
- The routing decision never calls an LLM or external service.
- Prompt content is not stored by default; the audit log records counts,
  classifications and decisions.

## Supported versions

Only the latest release on `main` receives security fixes.
