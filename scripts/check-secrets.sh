#!/usr/bin/env bash
# check-secrets.sh — fail if any git-tracked file contains a real secret.
#
# ISSUE-112: the repo is public, so this runs in CI on every PR. It looks for
# DISTINCTIVE real-secret formats only — never every mention of a key env var.
# Dev defaults (local_router_key, demo, <strong-secret>, $KEY, ${VAR:-}) are
# intentionally public and must not trip it. The classifier's own short
# detection tokens ("-----begin rsa" in internal/evals) are not full PEM
# headers and do not match either.
#
# Usage: scripts/check-secrets.sh        (exit 0 = clean, 1 = findings)

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Full PEM private-key headers, OpenRouter keys, Sluss admin keys, Z.ai-shaped
# keys (32 hex + '.' + 16 alnum), generic 'sk-' provider keys with long bodies,
# GitHub tokens, and Slack tokens.
PATTERN='-----BEGIN (RSA|OPENSSH|EC|PGP|DSA) PRIVATE KEY-----|sk-or-v1-[A-Za-z0-9]{20,}|sk_sluss_admin_[A-Za-z0-9]{10,}|[a-f0-9]{32}\.[A-Za-z0-9]{16,}|sk-(proj|ant)-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9]{30,}|xox[baprs]-[A-Za-z0-9-]{20,}'

HITS=0
while IFS= read -r f; do
  case "$f" in
    *.png|*.jpg|*.jpeg|*.gif|*.pdf|*.pptx|*.zip|*.ico|*.svg|*.woff|*.woff2) continue ;;
    scripts/check-secrets.sh) continue ;;   # this file documents the patterns
    *_test.go) continue ;;                   # test fixtures for the secret masker are deliberately fake keys
  esac
  # -e marks the pattern explicitly so a leading '-----' is not read as a flag.
  if grep -HnE -e "$PATTERN" "$f" 2>/dev/null; then HITS=1; fi
done < <(git ls-files)

if [ "$HITS" -ne 0 ]; then
  echo "SECRET SCAN FAIL — remove the secret, rotate it, and never commit it." >&2
  exit 1
fi
echo "secret scan: clean ($(git ls-files | wc -l) tracked files)"
