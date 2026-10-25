# Stop ChatGPT and show the company chat instead

**What you get:** when someone opens chatgpt.com, claude.ai or a similar site on a company computer, it does not open. Instead they see a short page that says the company has its own AI chat, with a button to it. The company chat is also on every home screen and in the browser's bookmarks.

**What you need:** the tool your company already uses to filter websites. That is often Microsoft Defender, Cloudflare, Cisco Umbrella or Zscaler. You also need the tool that manages company computers and phones (Intune, Jamf or similar).

**Time:** about 30 minutes.

## Steps

1. Open [Rollout]({{BASE}}/router/rollout) in the console and go to step 2.
2. Download the block list `genai-domains.csv`. It lists the AI sites to block and the address of this instance to keep open.
3. Under *Step by step, for the filter you run*, open the guide for your filter and follow it. It tells you where to upload the list and where to set the block page. The block page address is already filled in: `{{BASE}}/blocked`.
4. Download `sluss-webclip.mobileconfig` and `sluss-browser-policy.json`. Give them to whoever manages your computers, or upload them in Intune or Jamf yourself. They add the company chat to home screens and bookmarks, and they stop the browser from going around the filter.

## How to check

On a company computer, open chatgpt.com.

- With Cloudflare, Umbrella or Zscaler you should land on the Sluss block page, with a button to the company chat. You can see it here: [preview]({{BASE}}/blocked?from=chatgpt.com).
- With Microsoft Defender you see Microsoft's own block notice. Defender cannot send people to another page, so the bookmark and the home-screen icon show the way instead.

## Good to know

- This only covers computers and phones the company manages. A private phone on mobile data can still reach ChatGPT. That is a matter for your AI policy, not for technology.
- Sluss never sees ChatGPT traffic. Your filter does the blocking. Sluss supplies the list, the page people land on, and the proof that it works.
- Microsoft 365 Copilot is not on the list on purpose. Most companies already have an agreement for it.
