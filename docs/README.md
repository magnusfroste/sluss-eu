# Documentation map

Sluss has two kinds of documentation. The rule for which goes where:

| | `manual/` | `docs/` |
|---|---|---|
| **For** | Whoever runs Sluss: an IT administrator, a CISO, a self-hoster, or their agent | Whoever builds Sluss, or wants to understand how and why it works |
| **Answers** | *How do I…?* — set up models, stop ChatGPT, show an auditor | *How does it work, and why is it like this?* |
| **Written** | Plain English, one goal per page, no code knowledge needed | Engineering English (and some Swedish), precise |
| **Served** | On every instance at `/manual`, with that instance's address filled in; raw Markdown at `/manual/<page>.md` (start at `/manual/index.md`) | In the repository only |

When a feature ships, the manual says how to use it; `docs/` says how it is built. A page never needs both — link instead.

## In `docs/`

| Folder | What |
|---|---|
| `00-product/` | What Sluss is, positioning, NIS2 controls, market research |
| `01-architecture/` | System design: request lifecycle, routing engine, policy engine, security |
| `02-adr/` | Architecture decisions and their reasons |
| `03-backlog/`, `05-issues/` | Epics and issues — the backlog as shipped; `05-issues/issue-index.md` is the overview |
| `06-engineering/` | References, including the policy DSL |
| `07-operations/` | Deploying and running: [self-host with Docker](07-operations/self-host-docker.md), runbook, metrics, release checklist |
| `agents/` | Conventions for coding agents working in this repository |

## For agents

- **Installing or running Sluss** (not changing it): start with [`AGENTS.md` → Deploying or operating Sluss](../AGENTS.md#deploying-or-operating-sluss).
- **Changing the code:** the rest of [`AGENTS.md`](../AGENTS.md).
- An index of everything an agent may want to read is in [`llms.txt`](../llms.txt).
