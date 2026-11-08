# Show that it works

**What you get:** numbers and documents you can show to management, an auditor or a supervisory authority. They show where the AI data went, what was stopped, and that the company's computers cannot reach AI services directly.

**What you need:** admin access to this instance. For the per-device check you also need the tool that manages company computers.

**Time:** about 10 minutes, plus one day for the device reports to come in.

## Where the data went

Open the [Dashboard]({{BASE}}/router/dashboard). The top row shows how many requests stayed on your own models, how many were stopped, and how many contained personal data. Only the kind of data is shown, never the data itself.

## Why a single request went where it went

Open [Request log]({{BASE}}/router/log) and click a row. It shows which rules matched and which model answered. If the person signed in with the company account, it also shows who asked.

## That company computers cannot reach AI services directly

1. Open [Rollout]({{BASE}}/router/rollout), step 3, and download `coverage.ps1` for Windows or `coverage.sh` for Mac and Linux.
2. Run it daily through your management tool. On Windows that is an Intune *remediation*. On Mac it is a Jamf extension attribute.
3. The next day, the Dashboard shows *Managed devices: N / M still reach AI providers directly*, and Rollout lists every device. Each report contains only the computer's name, its operating system and two yes/no answers. It never contains a person or any content.

## Documents for an auditor

- **Audit log:** the tamper-evident record of every decision and every admin change. Download it from the console. The auditor checks it on their own computer with the free `audit-verify` tool: if a single line was changed, the check fails. When it fails, the tool also says why, line by line: a changed record, a removed record, or two Sluss processes that wrote at the same moment (which versions before v1.2 could do during an overlapping deploy).
- **The anchor:** a chain proves that nothing inside it was changed. It cannot prove that someone with access to the disk did not rewrite the whole file from a chosen point and recompute every link after it. For that, Sluss writes the chain's head (the number and hash of the last record) to its log every hour, as a line that says *audit anchor*. Keep the log where that person cannot reach it: your log service, a SIEM, another machine. An auditor then runs `audit-verify -anchor <seq>:<hash> audit-chain.jsonl` with any anchor from before, and a rewritten file fails. The current anchor is also at `{{BASE}}/router/audit/anchor`, for a script that files it somewhere else, for example once a day.
- **Answers that held what the question did not:** every answer is read for keys and personal identifiers that were not in the question. A finding is a *response.flagged* entry in the audit log, with the types and never the values, so you can show that what comes back is checked as well as what goes out.
- **Incident evidence:** one page with everything needed for the 24-hour and 72-hour reports that NIS2 asks for.
- **Control report:** the active rules and what they did, ready to attach to a report.

> These documents show what Sluss controlled and recorded. They are evidence for your own assessment, not a certificate that you comply with a law.

## Which version runs, and how to upgrade

The version in the menu footer opens the [Version]({{BASE}}/router/system) page. It shows the commit this instance runs, whether a newer version exists and what it changes, and the upgrade step for the way you deployed Sluss. A yellow dot next to the version means an update is waiting.

