# CI/CD

## CI-pipeline

Två workflows i `.github/workflows/`:

**`ci.yml`** (varje push/PR):

1. `go vet` + gofmt-check.
2. `go test ./...` (unit + integration med mock provider).
3. `make test-policy` (policy golden cases).
4. `make test-eval` (eval smoke).
5. `make demo-ciso` + `make eval-report` med artefakt-uppladdning.

**`publish-image.yml`**: bygger och pushar container-imagen till
`ghcr.io/${{ github.repository }}`.

## Policy CI

Policyändringar ska testas separat:

- Syntaxvalidering.
- Kompilering.
- Golden cases.
- No blocked model references.
- No orphan providers.

## Release

Release ska innehålla:

- Router version.
- Policy version.
- Registry version.

## Rollback

Rollback måste kunna ske för:

- Kod.
- Policy.
- Registry.

Policy rollback ska vara snabbast och kunna göras utan koddeploy.

## Environments

- `local`
- `dev`
- `staging`
- `production`

## Deployment checklist

- Policy aktiverad.
- Registry validerat.
- Provider keys tillgängliga.
- Smoke test passerat.
- Metrics syns.
- Alerts aktiva.
