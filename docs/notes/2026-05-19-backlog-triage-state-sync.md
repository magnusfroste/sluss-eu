# 2026-05-19 - Backlog Triage State Sync

## Context

The repo had a large canonical issue backlog in `05-issues/`, while `.ai/tasks.json` only tracked the newest post-triage work. The issue files also carried planning labels such as `priority`, `type`, and `sprint`, but most had no triage `state`.

## What I Learned

Backlog readiness can drift when `.ai` is bootstrapped after the original issue set already exists. A local issue may look specified enough for execution, but `continue` and next-task selection still depend on canonical `.ai` state on `main`.

On 2026-06-12, `.ai/state.json.next` still pointed at `ISSUE-044` even though issue files `ISSUE-044..052`, `ISSUE-056`, and `ISSUE-060` were already marked `state: done` with implementation closeouts. The fix was a reconciliation-only `.ai` update that marked those tasks done and advanced `next` to `ISSUE-063`.

## Reuse Rules

- When triaging the whole backlog, update both `05-issues/` labels and `.ai/tasks.json` in the same pass.
- Treat implemented code plus focused tests as stronger done evidence than sprint numbering alone.
- After expanding `.ai/tasks.json`, set `.ai/state.json.next` to the earliest ready non-done task unless the maintainer chooses a different lane.

## Failure Signals

- Most issue files have no `state:` label.
- `.ai/tasks.json` contains only recently created tasks while `05-issues/` has older implementation tickets.
- `.ai/state.json.next` points to a later issue even though earlier P0 issues are still unimplemented.
- `.ai/state.json.next` points to an issue whose file is already `state: done` with an implementation closeout.

## Next Checklist

- Count issue states after triage.
- Validate `.ai` JSON.
- Run the narrow or full test suite if done/ready decisions rely on current code evidence.
