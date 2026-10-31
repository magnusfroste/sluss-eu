# Sluss manual

Sluss is your organisation's AI gateway. Every question an employee or a tool sends to an AI model passes Sluss first. Sluss reads it with fixed rules, never with another AI. It then decides where it may go: harmless questions to a cheap cloud model, personal or confidential data to a model on your own hardware, and nothing at all when no allowed model exists. Every decision is written to a tamper-evident log you can hand to an auditor.

This manual is written for the person who sets Sluss up: an IT administrator, a security officer, or whoever drew the short straw. You do not need to know how identity providers, device management or DNS filters work. Each page says what it gives you, what you need, and then the steps, with this instance's own addresses filled in.

## Start here

If Sluss is new, do these in order. Each one works on its own, so you can stop after any of them.

1. [Set up the models and the first rule](/manual/quick-start), about 10 minutes. After this, Sluss answers questions and keeps personal data local.
2. [Let employees sign in with the company account](https://github.com/magnusfroste/sluss-eu/blob/main/docs/07-operations/self-host-docker.md#let-employees-sign-in-with-the-company-account), about 20 minutes. After this, every question is logged with a name and a department, and nobody needs a new password.
3. [Stop ChatGPT and show the company chat instead](/manual/stop-chatgpt), about 30 minutes. After this, people who open chatgpt.com on a company computer land in Sluss.
4. [Make developer tools go through Sluss](/manual/developer-tools), about 15 minutes. After this, coding tools and scripts use Sluss without anyone changing a setting.
5. [Show that it works](/manual/prove-it), about 10 minutes. After this, you have numbers and documents for management, an auditor or a supervisory authority.

## When you need it

- [How Sluss picks a model](/manual/choosing-models): the three steps, what a model's level means, and why a question went where it went.
- [Words used in this manual](/manual/glossary): short explanations of IdP, MDM, OIDC, DNS filter and the rest.
- [When something does not work](/manual/troubleshooting): the errors people actually hit, and what to do.

## This instance

- Address: `{{BASE}}`
- Admin console: [{{BASE}}/router/dashboard]({{BASE}}/router/dashboard)
- Company AI chat: [{{BASE}}/chat]({{BASE}}/chat)

> Agents and scripts can read every page as plain Markdown: add `.md` to the address, for example `{{BASE}}/manual/quick-start.md`.
