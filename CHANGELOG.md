# Changelog

All notable changes to Sluss. Versions follow [semantic versioning](https://semver.org): a patch release fixes, a minor release adds, a major release may change how you configure or run Sluss.

Releases are published from this file by the `release` workflow (Actions → release → Run workflow). Every release is a container image you can pin:

```
ghcr.io/magnusfroste/sluss-eu:v1.2.2   # this release
ghcr.io/magnusfroste/sluss-eu:1.2      # the newest 1.2.x
ghcr.io/magnusfroste/sluss-eu:latest   # main, for trying things out
```

## Unreleased

### Protection
- **More identifiers are caught.** Phone numbers as people write them (`070-123 45 67`, `+46 70 …`), Swedish postal addresses (a street with a house number and a postcode), sick leave, burnout and chemotherapy as health words, and code pasted without fences.
- **What the rules catch is measured and published.** A fixed set of 46 examples runs with every change (`internal/classifier/catch_corpus_test.go`); the manual page *What Sluss catches, and what it does not* carries its numbers and the documented misses, and the test fails if the page falls out of date.

### Positioning
- **The AI gateway whose decisions you can read.** README and the manual now say why the rules are not another AI, what that costs, and how to close the gap.

### Fixed
- **The chat shows the built-in test answer.** The mock could not stream, so the chat rendered nothing until a real model was connected, although the API answered. The mock now streams like a provider.

### Supply chain
- **Signed images and a bill of materials.** Every image is signed with Sigstore (keyless, bound to the build workflow's identity) and carries an SPDX SBOM as an attestation; releases have the SBOM attached as a file. `cosign verify` checks both; the self-host guide shows how.
- **Known vulnerabilities stop a merge.** CI runs `govulncheck` on every change. Sluss is built with Go 1.26.10; the 1.25 line is no longer maintained and carried twelve standard-library vulnerabilities the code calls.

### Evidence
- **The audit chain is anchored outside the machine.** A hash chain cannot by itself show that someone with disk access did not rewrite the file from a chosen point and recompute every later hash. Every hour Sluss now logs an `audit anchor` line with the last record's sequence number and hash; shipped with the log to a SIEM, that line is the proof. `audit-verify -anchor <seq>:<hash>` checks an export against it, and `GET /router/audit/anchor` returns the current anchor. `ROUTER_AUDIT_ANCHOR_INTERVAL` sets the interval.

### Security
- **The login form brakes password guessing.** After five wrong passwords for an account (twenty for a client address) within fifteen minutes, further attempts are refused for a wait that doubles each time, up to fifteen minutes. Every wrong password and every brake is in the audit log (`console.login_failed`, `console.login_throttled`), with the address it came from.
- **Browser hardening headers on every response:** a Content-Security-Policy that allows this origin only, no framing, no content sniffing, a referrer policy and a permissions policy. `Strict-Transport-Security` is sent behind a TLS-terminating proxy.
- **`/metrics` needs a key.** The console gate opens it (a signed-in admin, an admin key, the break-glass password), and `ROUTER_METRICS_TOKEN` is a separate bearer token for the scraper, so Prometheus never holds an admin key. The new `GET /version` says which build runs, with no key and no secret in it; the deploy check uses it.

## v1.2.2 — 2026-11-06

### Documentation
- **A model server on the same machine as the container.** Inside the container, `localhost` is the container itself, so Ollama or vLLM on the host was unreachable when given `http://localhost:11434`. The self-host guide's run command and the Compose file now add `host.docker.internal`, and Quick start and troubleshooting say which address to use and that the server must listen on all interfaces.

## v1.2.1 — 2026-11-05

A fresh install works as the documentation says.

### Fixed
- **Your own models work without an OpenRouter key.** Before, the models added in the console were only used when `OPENROUTER_API_KEY` was set, so an install with only local hardware (or no internet) never routed to them. A provider connection may now leave the key variable empty, for a local server that takes no key.
- **A container with no provider answers.** The documented built-in test answer did not exist in the image: the first question failed with a connection error. Until a real model is connected, every answer now says plainly that no model is connected yet.
- **`docker compose up` works without the search profile.** `SEARXNG_SECRET` was required even when SearXNG was not started.

### Documentation
- The README try-out needs no second terminal, routes the personal-data example locally, and shows how to open the console.
- The self-host guide pins v1.2.1, explains the three ways into the console (named accounts, the company account, break-glass), and tells air-gapped installs which two outbound checks to turn off.
- `deploy/example.env` and the Compose file carry the settings they lacked (shadow policy, profile, MCP, pricing sync, banner) and describe the break-glass login correctly.
- The manual says who can sign in, what to do when locked out, that the client secret is optional for single sign-on, and that `/metrics` needs no sign-in.

## v1.2.0 — 2026-11-01

Sharper protection, a trustworthy audit chain, and Sluss as a multi-user service.

### Protection
- **A pasted key is caught.** A prompt that *contains* an API key, token or private key (AWS, `sk-…`, GitHub, Slack, PEM, `aws_secret_access_key`) is classified as secrets, even when it never says "key". Before, a pasted AWS key could reach a cloud model.
- **Swedish and NATO/EU security markings.** The protection levels and the law (*begränsat hemlig*, *säkerhetsskyddslagen*, …), stamps written in capitals (`HEMLIG`, `KONFIDENTIELL`, `KVALIFICERAT HEMLIG`), and NATO/EU markings classify as security-classified. Lower-case everyday words do not.
- **Rules match every kind of data in a question**, not only the strongest. A rule that blocks classified material now fires when the same question also holds a key. Stricter, never weaker. The dry-run and the inspector show every class found.

### Evidence
- **The audit chain cannot fork.** During an overlapping deploy two containers could append to the same chain, which then failed verification although nothing was changed. Each record now locks the file and continues from the record that is really last.
- **`audit-verify` explains a broken chain** line by line: a changed record, a removed record, or two writers at once — and says so plainly when every record is intact and none is missing.

### People
- **One chat history per person.** Before, admins, the break-glass login and demo-link guests shared one history. Each identity now has its own, and the browser copy is per person.
- **Your profile** (`/me`): who you are and how you signed in, change the password of a local account, download or delete your chat history, sign out. Reached from your name in the menu.
- **The console shows who is signed in**, and marks a login with the break-glass password.

### Docs
- New manual pages: **How Sluss picks a model** (the three steps, what a model's level means) and **Works with your stack** (web filters, an AI proxy you already run, chat apps).
- `docs/README.md` explains what lives in `manual/` and in `docs/`; `AGENTS.md` starts with a section for agents that install or operate Sluss; `llms.txt` indexes it all.

## v1.1.0 — 2026-10-31

A new chat, an inspector that shows what Sluss decided and what was sent, and a console that is easier to find your way around.

### Chat
- **A modern assistant layout.** History on the left, grouped by day and searchable; a centred message box on an empty chat; Enter sends and Shift+Enter starts a new line; Stop while an answer streams; files can be dropped in. Answers render as Markdown — code blocks with a copy button, tables, lists — through a renderer that escapes everything a model writes, so an answer can never inject markup or script. Copy on every answer, Regenerate on the latest.
- **Inspector beside the chat.** Each answer gets one short line in the flow (`● deepseek · stayed local · personal data`); the details sit in foldable cards on the right, each with a one-line summary when folded:
  - **This chat** — how many answers stayed local, went to the cloud or were blocked.
  - **Route** — task and how sure the classifier was, risk, kind of data, the policy rules that matched, model, provider, local or cloud, fallback.
  - **Context** — what was actually sent: messages, size, whether the data came from earlier in the chat, attachments, web sources.
  - **Output** — time to the first word, total time, tokens, cost, request id.
  - **Setup** — model, web search, who you are, which version runs.

  Employees see a plain version without rule ids, providers or costs.
- **A blocked question is never sent again.** The blocked turn is held back from later messages, so a secret in it does not ride along or keep steering the rest of the chat.

### Console
- **Model panel.** Click a model on Models: status, a live test, the provider's own model list with Use this, settings with explanations, and the last 24 hours of traffic and failures. "Needs you" on the Overview links straight to a failing model.
- **Connect clients** inside the console, with copy buttons and a key field that fills every recipe in the browser only.
- **The manual opens inside the console**, so the menu is one click away.
- **Clearer blocks.** When a request is blocked because no allowed model could take it, the reason now names the local models that were skipped and why (for example below the minimum tier for the task).

### Operations
- **Boot banner.** On start Sluss prints its version, commit, console address, data directory and update-check setting, plus one JSON line for log pipelines (`ROUTER_LOG_BANNER=off` for JSON only).
- New response headers for clients that want to show the routing: `X-Router-Risk`, `X-Router-Selected-Provider`, `X-Router-Policy-Rules`, `X-Router-Task-Confidence`, `X-Router-Prompt-Tokens` — classification and numbers only, never content.
- The image's version label now reads the release tag (`v1.1.0`).

## v1.0.0 — 2026-10-29

The first release: a self-hosted AI gateway an EU organisation can put between its people, its tools and AI models, with control before every call and evidence after it.

### Control before the call
- **Two wire formats.** OpenAI chat completions and Anthropic messages, with streaming and tool use, so existing SDKs, coding tools and agents work by changing one address.
- **Deterministic classification.** Every prompt is read by fixed rules, never by an AI: task type, risk, and kind of data — secrets, personal data (including Swedish personal identity numbers), security-classified, health, financial, legal, source code. Attached documents are classified as text; unreadable attachments are their own class.
- **Fail-closed policy.** Rules decide where each kind of data may go. When no allowed model exists, the request is stopped and recorded — never sent somewhere else to get an answer. Compliance packs for NIS2, DORA, GDPR and personal data, or rules clicked together in the console.
- **Agent tool gating.** Declared tools are classified read, write, external or destructive, and policy can keep a delete-capable agent on your own hardware.
- **Your own models.** Any OpenAI-compatible server (vLLM, Ollama, LM Studio, a DGX or RTX box) tagged local; personal data stays there. A live Test per provider shows what each server actually serves, and switches a model in one click when a lab box changes.

### Evidence after it
- **Hash-chained audit log** with an offline verifier, so an auditor can check that nothing was changed.
- **Incident evidence** for the NIS2 24-hour and 72-hour reports, a gap report for a shadow policy, and a control report.
- **Request log** that explains every decision in plain words: what it was, where it went, which rule decided, and — with company sign-in — who asked.

### Fits the organisation's IT
- **Sign in with the company account** over OpenID Connect (Microsoft Entra ID, Okta, Google, Keycloak). Sluss keeps no employee accounts; groups decide who administers and which department a request belongs to.
- **Rollout kit** for Intune, Jamf and Fleet: scripts that point developer tools at Sluss, a deny list and block page for the DNS filter or web gateway, step-by-step guides for Defender, Cloudflare, Umbrella and Zscaler, and a daily per-device check of whether AI services can still be reached directly.
- **Company AI chat** as the place people land instead of ChatGPT: file attachments, web search through your own SearXNG (skipped when a prompt holds sensitive data), and chat history per person.

### Runs anywhere
- **One container, one volume.** SQLite under `/data`, no external database. Docker, Docker Compose, EasyPanel or AgentHotel; works air-gapped.
- **A console that shows what needs you first**, in six sections, with a manual written for administrators without specialist training.
- **Version page** that says which release runs, whether a newer one exists and what it changes, and how to upgrade the way you deployed — Sluss never upgrades itself.
- **Hot reload** of models, providers and rules without a restart.

Open source under AGPL-3.0. A control and evidence tool, not a certificate of legal compliance.
