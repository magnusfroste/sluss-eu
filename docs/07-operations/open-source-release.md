# Open-source release — cutover runbook

> One-time owner procedure (ISSUE-112). The repository is public with a
> **fresh history**: the pre-cutover history contained third-party names that
> were later removed, and rewriting a shared history is riskier than starting
> clean. The old repository stays as a private, frozen archive.

## 1. Verify the tree is clean (run in the sanitised checkout)

```bash
bash scripts/check-secrets.sh          # secrets: must print "clean"
go build ./... && go test ./... -count=1
```

## 2. Create the public repository on GitHub

GitHub → New repository → name `sluss-eu`, **Public**, *no* README/licence/
.gitignore (the tree already has them). The old private `sluss` repository
stays as a frozen archive.

## 3. Push with fresh history

```bash
# from the sanitised working tree
rm -rf .git
git init -b main
git add -A
git commit -m "Sluss — data-sovereign LLM gateway (open-source release)"
git remote add origin git@github.com:magnusfroste/sluss-eu.git
git push -u origin main
```

## 4. Post-push checklist

- [ ] GitHub → Settings → General: confirm **Public**; enable Issues.
- [ ] GitHub → Security: enable **secret scanning** and **push protection**
      (belt and braces on top of `scripts/check-secrets.sh` in CI).
- [ ] Add repository topics: `llm-gateway`, `ai-gateway`, `nis2`, `dora`,
      `gdpr`, `data-sovereignty`, `agent-gateway`, `golang`.
- [ ] Verify the CI workflow ran green on the first commit.
- [ ] Point EasyPanel (or your deploy) at the new repo URL; redeploy.
- [ ] Update `ROUTER_PUBLIC_URL`-adjacent links if any pointed at the old repo.

## 5. Ongoing rules

- Anything about people, customers, prospects or partners lives **outside the
  repo** (private notes/vault), never in `docs/` or `.scratch/`.
- Secrets are env-var names only; CI fails the PR on real key formats.
- Product research may name public vendors as comparisons; it never records
  private conversations with them.
