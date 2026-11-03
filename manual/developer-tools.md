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
- Partly covered: GitHub Copilot in VS Code. Chat and agent mode can use Sluss, but inline suggestions still go to GitHub. See the next section.
- Not covered: Microsoft 365 Copilot and the ChatGPT app. Their own chat has no setting for another address. Block them, or cover them with a separate agreement. In Microsoft 365 you can only bring your own model through a custom engine agent or Copilot Studio, not to Copilot's own chat.

## GitHub Copilot in VS Code

Since VS Code 1.122 (May 2026), VS Code can send Copilot chat to any OpenAI-compatible address. The feature is called *Custom Endpoint*, and it is one of the *bring your own key* (BYOK) model providers. Sluss is such an address.

**What goes through Sluss:** the Chat view, agent mode, and the tools and MCP servers agent mode calls. Small background tasks, such as chat titles and commit messages, also go through Sluss if you pick the Sluss model for them (step 6).

**What still goes to GitHub:** inline suggestions (the grey text while you type), next edit suggestions, and semantic search of the code. VS Code cannot send these to your own model. Code that people type in the editor can still reach GitHub's service this way. If that is not acceptable, turn off inline suggestions in VS Code, or keep GitHub Copilot off those computers.

**Before you start, on Copilot Business or Enterprise:** a GitHub.com policy decides whether people may add their own models in VS Code. The VS Code documentation calls it *Bring Your Own Language Model Key in VS Code*. GitHub's documentation describes it as a policy that can turn the feature off. Ask the organization or enterprise owner on GitHub.com to check that it is on for the people who need it. It works both ways: while the policy is on, a person can also add a model that goes straight to OpenAI or Anthropic, past Sluss. Blocking those addresses in your web filter closes that path. See [Stop ChatGPT](/manual/stop-chatgpt).

**Steps, on each computer:**

1. Update VS Code to 1.122 or later.
2. In the Chat view, open the model picker and select *Manage Language Models* (the gear icon). You can also run the command *Chat: Manage Language Models*.
3. Select *Add Models*, then *Custom Endpoint*. VS Code opens the file `chatLanguageModels.json`.
4. Paste the entry below into the file and save it. It uses the full address, including `/v1/chat/completions`, and the model `auto`, which lets Sluss choose the model for each question.
5. When VS Code asks for `slussKey`, paste the department key from [Keys]({{BASE}}/router/keys). The `${input:slussKey}` form keeps the key out of the file.
6. Pick *Sluss (auto)* in the model picker. To send background tasks through Sluss too, set `chat.utilityModel` and `chat.utilitySmallModel` to the same model.

The entry for step 4:

```json
{
  "name": "Sluss",
  "vendor": "customendpoint",
  "apiKey": "${input:slussKey}",
  "apiType": "chat-completions",
  "models": [
    {
      "id": "auto",
      "name": "Sluss (auto)",
      "url": "{{BASE}}/v1/chat/completions",
      "toolCalling": true,
      "vision": false,
      "maxInputTokens": 120000,
      "maxOutputTokens": 8000
    }
  ]
}
```

- Keep `toolCalling` set to `true`. Without it, VS Code hides the model from agent mode.
- Set `vision` to `true` only if the models Sluss can route to accept images.
- Set the two token numbers so they fit the smallest context window among those models.
- VS Code keeps `chatLanguageModels.json` in each person's own settings. The rollout script does not set it, so this part is done by hand or with your own settings tooling.
- Since 1.122 this also works without signing in to GitHub. Inline suggestions then stop, because they need GitHub.

## How to check

Open [Request log]({{BASE}}/router/log). Requests from the tools show up with the department of the key they used. Click a row to see why it was sent where it went.

For GitHub Copilot, ask a question in the Chat view with *Sluss (auto)* picked. It shows up in the Request log like any other request.
