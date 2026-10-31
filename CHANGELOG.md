# Changelog

All notable changes to Sluss. Versions follow [semantic versioning](https://semver.org): a patch release fixes, a minor release adds, a major release may change how you configure or run Sluss.

Releases are published from this file by the `release` workflow (Actions → release → Run workflow). Every release is a container image you can pin:

```
ghcr.io/magnusfroste/sluss-eu:v1.1.0   # this release
ghcr.io/magnusfroste/sluss-eu:1.1      # the newest 1.1.x
ghcr.io/magnusfroste/sluss-eu:latest   # main, for trying things out
```

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
