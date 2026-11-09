# One address, closed doors

**What you get:** the one idea that makes Sluss work, and the part most people get wrong at first. Sluss is not something you put *in* the network. It is an address you give your AI tools, and the doors around it are closed by tools you already have.

**What you need:** nothing to install. This page explains how the pieces fit.

**Time:** 5 minutes to read.

## Sluss is an address, not a proxy

A network proxy sits in the traffic and sees everything that passes. Sluss does not. Nothing is intercepted, no certificate is installed on anyone's computer, and Sluss never sees traffic that is not sent to it.

Instead, Sluss answers in the same way an AI service answers. Every AI tool that lets you choose its server, which is almost all of them, gets Sluss's address instead of the AI company's:

| The tool | What you change |
|---|---|
| The company chat | Nothing: people open Sluss's own chat in the browser. |
| Coding tools (Cursor, Claude Code, Codex, Continue, VS Code) | The *base URL* setting, and a key. |
| Chat apps and AI platforms (Open WebUI, AnythingLLM, LibreChat) | The *OpenAI-compatible address* setting, and a key. |
| Your own apps and scripts | `OPENAI_BASE_URL` or `ANTHROPIC_BASE_URL`, and a key. |
| AI agents | The same base URL; their tools are declared in the request, and Sluss judges what the agent may do. |

From then on every question goes to Sluss first. Sluss reads it, decides where it may go, sends it there or stops it, and writes the decision to its log. The answer comes back the same way. To the tool, Sluss *is* the AI service.

[Connect clients]({{BASE}}/router/connect) has the exact setting for each tool.

## Then close the other doors

An address only helps if it is the only way to AI. Someone could still open chatgpt.com in a browser, or install an app that talks to an AI company directly. Sluss does not stop that, and it should not try: that is the job of tools your IT already runs. Each one closes a door.

| Door | The tool that closes it | How |
|---|---|---|
| **Who may use AI at all** | Your identity provider: **Microsoft Entra ID**, Okta, Google, Keycloak | People sign in to Sluss with the company account. Entra's Conditional Access can require a managed, compliant device, and MFA. Someone who leaves loses access the moment their account is closed. |
| **Which settings a computer has** | Your device management: **Microsoft Intune**, Jamf, Kandji, Fleet | Intune pushes the scripts from [Rollout]({{BASE}}/router/rollout): they point every tool with a base URL at Sluss, put the company chat on every home screen, and set a browser policy. People do not have to change anything themselves. |
| **Which AI websites can be reached** | Your DNS filter or web gateway: **Microsoft Defender**, Zscaler, Netskope, Cisco Umbrella, Cloudflare | They block chatgpt.com, claude.ai and the rest, and send people to Sluss's block page, which points to the company chat. Rollout has the deny list. |
| **Whether a computer can still go around** | Sluss's coverage check, run by Intune | Once a day, a read-only script on each computer tests whether an AI service can be reached directly. The dashboard shows how many can. |

Together: the tools that need AI have Sluss's address, the sites that do not go through Sluss are closed, and you can see, per device, that it holds. **The only AI path is Sluss.**

## Why it is built this way

- **Nothing new on the computers.** No agent, no certificate, no traffic inspection. Sluss runs as one service; the computers are managed by what you already use.
- **Nothing in the way of everything else.** Sluss only sees what is sent to it: AI questions. E-mail, files and the rest of the network are untouched.
- **Each tool does what it is best at.** The identity provider decides who, device management decides how computers are set up, the DNS filter decides which sites open, and Sluss decides where each AI question may go and proves it.

## Where to start

1. [Set up the models and the first rule](/manual/quick-start), so Sluss answers.
2. [Make developer tools go through Sluss](/manual/developer-tools): the address, pushed by Intune.
3. [Stop ChatGPT and show the company chat instead](/manual/stop-chatgpt): close the website door.
4. [Show that it works](/manual/prove-it): the coverage per device.

[Works with your stack](/manual/works-with-your-stack) shows where Sluss sits next to tools you may already have, such as a web filter or an AI proxy.
