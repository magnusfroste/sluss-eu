# Sluss manual

Sluss is your organisation's AI gateway. Every question an employee or a tool sends to an AI model passes Sluss first. Sluss reads it with fixed rules, never with another AI. It then decides where it may go: harmless questions to a cheap cloud model, personal or confidential data to a model on your own hardware, and nothing at all when no allowed model exists. Every decision is written to a tamper-evident log you can hand to an auditor.

Sluss decides with rules you can read, not with another AI. That is the point: the same question gives the same decision every time, which is what evidence needs, and nothing leaves your machine to be classified. It is also a limit: rules find identifiers, keys, markings, terms and code, not meaning. [What Sluss catches](/manual/what-sluss-catches) measures it honestly. If you want more, a small model on your own hardware can give a second opinion that may only make a decision stricter; it is off until you turn it on, and [How Sluss decides](/manual/how-sluss-decides) shows how to prove which way it was.

This manual is written for the person who sets Sluss up: an IT administrator, a security officer, or whoever drew the short straw. You do not need to know how identity providers, device management or DNS filters work. Each page says what it gives you, what you need, and then the steps, with this instance's own addresses filled in.

**First, the one idea.** Sluss is not put *in* your network. It is an address you give your AI tools, and the doors around it are closed by tools you already have: Entra ID decides who, Intune sets up the computers, your DNS filter closes the AI websites. [One address, closed doors](/manual/one-address) explains it in five minutes, and everything below builds on it.

## Start here

If Sluss is new, do these in order. Each one works on its own, so you can stop after any of them.

1. [Set up the models and the first rule](/manual/quick-start), about 10 minutes. After this, Sluss answers questions and keeps personal data local.
2. [Let employees sign in with the company account](https://github.com/magnusfroste/sluss-eu/blob/main/docs/07-operations/self-host-docker.md#let-employees-sign-in-with-the-company-account), about 20 minutes. After this, every question is logged with a name and a department, and nobody needs a new password.
3. [Stop ChatGPT and show the company chat instead](/manual/stop-chatgpt), about 30 minutes. After this, people who open chatgpt.com on a company computer land in Sluss.
4. [Make developer tools go through Sluss](/manual/developer-tools), about 15 minutes. After this, coding tools and scripts use Sluss without anyone changing a setting.
5. [Show that it works](/manual/prove-it), about 10 minutes. After this, you have numbers and documents for management, an auditor or a supervisory authority.

## When you need it

- [Works with your stack](/manual/works-with-your-stack): where Sluss sits next to your web filter (Zscaler, Defender and the rest), an AI proxy you already run, and chat apps.
- [How Sluss picks a model](/manual/choosing-models): the three steps, what a model's level means, and why a question went where it went.
- [Rules only, or a second opinion](/manual/how-sluss-decides): the two ways Sluss can decide, which fits you, and how to show which one was in use.
- [What Sluss catches, and what it does not](/manual/what-sluss-catches): the rules, measured on a fixed set of examples, and how to close the gap.
- [Where each setting lives](/manual/where-settings-live): what is changed in the console and what is changed by whoever runs the server, and why a new key needs a restart while a new model does not.
- [Words used in this manual](/manual/glossary): short explanations of IdP, MDM, OIDC, DNS filter and the rest.
- [When something does not work](/manual/troubleshooting): the errors people actually hit, and what to do.

## This instance

- Address: `{{BASE}}`
- Admin console: [{{BASE}}/router/dashboard]({{BASE}}/router/dashboard) (the menu calls it *Overview*)
- Company AI chat: [{{BASE}}/chat]({{BASE}}/chat)

## Who can sign in

- **Employees** sign in with the company account (step 2 above) and get the chat. Each person has their own chat history.
- **Administrators** get the console: members of the admin groups at the identity provider, or local accounts managed under [Console users]({{BASE}}/router/users). The first local account is created from the server's environment when Sluss starts for the first time.
- **You** can see who you are signed in as, change the password of a local account, and download or delete your own chat history on [Your profile]({{BASE}}/me), reached from your name at the bottom of the menu.
- **Locked out?** Whoever runs the server has a break-glass password; see [When something does not work](/manual/troubleshooting).

> Agents and scripts can read every page as plain Markdown: add `.md` to the address, for example `{{BASE}}/manual/quick-start.md`.
