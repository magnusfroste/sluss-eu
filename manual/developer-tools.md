# Make developer tools go through Sluss

**What you get:** coding assistants, AI libraries and scripts on company computers send their questions to Sluss instead of straight to OpenAI or Anthropic. Nobody has to remember a setting, and the same rules apply to them as to the chat.

**What you need:** the tool that manages company computers (Intune, Jamf, Fleet or similar), and an API key for each department from the [Keys]({{BASE}}/router/keys) page.

**Time:** about 15 minutes.

## Steps

1. Open [Keys]({{BASE}}/router/keys) and create a key for each department, for example *finance* and *engineering*. Each key is shown once, so copy it to a safe place right away.
2. Open [Rollout]({{BASE}}/router/rollout), step 1, and download the script for your computers: `sluss-env.ps1` for Windows, `sluss-env.sh` for Mac and Linux.
3. Run the script once on every computer through your management tool. On Windows that is an Intune *platform script* in system context. On Mac it is a Jamf policy. The script tells the tools on the computer to use `{{BASE}}`.
4. Hand out the department key through your management tool's secret handling. Never put it in the script itself.

## Which tools this covers

- Covered: the OpenAI and Anthropic libraries, Claude Code, Continue, Aider, and most scripts and agents. In short, anything that reads `OPENAI_BASE_URL` or `ANTHROPIC_BASE_URL`.
- Set by hand: tools that keep the address in their own settings, such as Cursor. Use the recipes on [Connect]({{BASE}}/connect).
- Not covered: GitHub Copilot, Microsoft 365 Copilot and the ChatGPT app. They have no setting for another address. Block them, or cover them with a separate agreement.

## How to check

Open [Request log]({{BASE}}/router/log). Requests from the tools show up with the department of the key they used. Click a row to see why it was sent where it went.
