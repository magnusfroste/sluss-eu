# 2026-05-19 - Classifier keyword boundaries

## Context

ISSUE-016 added deterministic prompt feature extraction for code, path, stack trace, tool, JSON schema, and risk keyword signals.

## What I Learned

Risk keywords that are short substrings can create false positives in normal project terms. For example, matching `token` with raw substring logic would classify `tokenizer` as an auth or secret signal.

## Reuse Rules

- Match single-word classifier keywords with word boundaries.
- Use substring matching only for multi-token phrases or path-like/file-like terms such as `alter table`, `.sql`, or `db/migrations`.
- Add explicit related words such as `authentication` or `authorization` instead of relying on broad prefix or substring matching for `auth`.

## Failure Signals

- Benign repo, package, or product names unexpectedly trigger auth, secret, payment, or security hints.
- A low-risk prompt receives high-risk sensitivity hints without an obvious keyword in the actual user wording.

## Next Checklist

- When adding new classifier keywords, check whether the term can appear inside common product or package names.
- Add a focused regression case before widening keyword matching rules.
