# Learning-note authoring protocol

Open when creating or revising a reusable learning note or bootstrapping the notes structure. Paths in the protocol below are relative to this repository root.

## Autonomous Learning

### Goal

Continuously improve by writing reusable project learnings without waiting for user prompts.

### Mandatory Behavior

1. On the first substantive task in this repo, ensure these files exist:
   - `AGENTS.md`
   - `docs/notes/index.md`
   - `docs/notes/_template-learning-note.md`
2. At the end of every substantive task, run a lesson check autonomously.
3. If a reusable lesson exists, create or update a note in `docs/notes/` and update `docs/notes/index.md` in the same change.
4. If no reusable lesson exists, explicitly include `no reusable lesson` in the final summary.
5. Do not wait for the user to ask for notes or scripts.

### Lesson Check

Write or update a note if any of these happened:

- an error pattern or non-obvious bug
- an architecture tradeoff
- a performance behavior change caused by a fix

### Meaningful Lesson Filter

Write notes for:

- root-cause patterns
- regressions and preventions
- state-sync pitfalls
- performance and UX behavior rules

Do not write notes for:

- trivial copy edits
- formatting-only changes
- mechanical renames with no new insight

### Note Format

Use this structure:

```markdown
# YYYY-MM-DD - <topic>

## Context

## What I Learned

## Reuse Rules

## Failure Signals

## Next Checklist
```

### Notes Index Format

Keep `docs/notes/index.md` optimized for retrieval, not just chronology.

When creating or updating notes:

- Keep note files as individual markdown files in `docs/notes/`.
- Do not create deep subfolders unless the repo already uses them.
- Update `docs/notes/index.md` as a routing map with topic groups that match this repo's domains and tooling.
- Use only categories relevant to this repo, such as `Routing`, `Policy`, `Providers`, `Auth`, `Testing`, `Operations`, `Performance`, or `Security`.
- For each note entry, include date, title, link, and short retrieval cues such as keywords, failure signals, or "open when..." guidance.
- Keep a short `High-signal recurring lessons` section near the top for notes that should often be checked before related work.
- Keep a chronological section or table if useful, but do not make it the only navigation path once the notes list grows.
- During any index update, remove or flag broken links and stale entries for notes that no longer exist.
