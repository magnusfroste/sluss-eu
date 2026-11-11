# Self-host Sluss with Docker

Sluss is one container and one data volume. It needs no database server and no Redis. Everything it keeps, including tenants and keys, the model roster, console rules, request history and the hash-chained audit log, lives in a SQLite file under `/data`.

The image is public. Releases are listed in [CHANGELOG.md](../../CHANGELOG.md):

| Tag | Use |
|---|---|
| `ghcr.io/magnusfroste/sluss-eu:v1.11.0` | Production: pin a release |
| `ghcr.io/magnusfroste/sluss-eu:1.11` | Production: the newest 1.11.x, patch fixes included |
| `ghcr.io/magnusfroste/sluss-eu:latest` | Trying it out: rebuilt on every merge to `main` |
| `ghcr.io/magnusfroste/sluss-eu:sha-<short>` | Pin the exact build you reviewed |

Set `ROUTER_DEPLOYMENT=docker` (or `compose`, `easypanel`, `agenthotel`) so the Version page shows your upgrade step first.

### Check the image before you run it

Every image is signed with Sigstore, keyless: the signature is bound to the build workflow's identity at GitHub, so there is no key that could leak. Each image also carries a software bill of materials (SPDX) as an attestation, and each release has it attached as a file. Both are checked with [cosign](https://github.com/sigstore/cosign):

```bash
cosign verify ghcr.io/magnusfroste/sluss-eu:v1.11.0 \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/magnusfroste/sluss-eu/\.github/workflows/publish-image\.yml@refs/tags/v'

cosign verify-attestation --type spdxjson ghcr.io/magnusfroste/sluss-eu:v1.11.0 \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/magnusfroste/sluss-eu/' \
  | jq -r .payload | base64 -d | jq .predicate > sbom.spdx.json
```

The first command fails unless the image was built by this repository's release workflow from a version tag. The second gives you the bill of materials to feed your own vulnerability scanner. CI runs `govulncheck` on every change, so a known vulnerability that the code actually calls stops a merge, and the Go patch level is kept current.

## 1. Run it

```bash
docker volume create sluss-data

docker run -d --name sluss --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v sluss-data:/data \
  --add-host=host.docker.internal:host-gateway \
  -e LOCAL_API_KEY="$(openssl rand -hex 24)" \
  -e ROUTER_ADMIN_USER=admin \
  -e ROUTER_ADMIN_PASSWORD='choose-a-long-passphrase' \
  -e ROUTER_POLICY_PATH=builtin:nis2-baseline \
  -e OPENROUTER_API_KEY='…' \
  ghcr.io/magnusfroste/sluss-eu:v1.11.0
```

- **`-v sluss-data:/data` is the part you must not skip.** The volume holds the audit chain. Without it, everything is lost when the container is recreated.
- **`--add-host=host.docker.internal:host-gateway`** lets the container reach a model server on the same machine, for example Ollama or vLLM on the host. Inside the container, `localhost` is the container itself, so give such a server the address `http://host.docker.internal:11434` in the console, and make sure it listens on all interfaces (Ollama: `OLLAMA_HOST=0.0.0.0`; it listens on `127.0.0.1` only by default). A server on another machine is reached by its ordinary address.
- **`LOCAL_API_KEY`** is the bootstrap Bearer key. Once you have created per-department keys in the console, clients should use those instead.
- **`ROUTER_ADMIN_USER` / `ROUTER_ADMIN_PASSWORD`** seed the first named console user, but only while no user exists. After that, the account lives in `/data`: change the password on **Your profile** (`/me`), not by editing these variables. See *Console accounts* below.
- **`ROUTER_POLICY_PATH`** selects a built-in compliance pack: `builtin:nis2-baseline`, `builtin:dora-baseline`, `builtin:gdpr-sovereign` or `builtin:pii-local`.
- **`OPENROUTER_API_KEY` is optional.** Leave it out when every model runs on your own hardware: add those models in the console instead (step 2). Until a real model is connected, every question gets a built-in test answer that says so, so a first look works without any key.

The settings an operator needs are in [`deploy/example.env`](../../deploy/example.env); the development and experimental ones are in [`.env.example`](../../.env.example).

Prefer compose? [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) sets up the same service. Swap `build:` for the `image:` line and put your values in a `.env` next to it.

## 2. Set it up

Open `http://localhost:8080/router/setup` and sign in. **Quick start** walks you through four steps:

1. A cloud model.
2. A local model, for example vLLM or Ollama on your own GPU, tagged `local`. A server that takes no API key needs no key variable: leave that field empty. If the server runs on the same machine as the container, its address is `http://host.docker.internal:<port>`, not `localhost` (see the run command above).
3. One rule that keeps personal data local.
4. A dry run that shows where a prompt would go.

Point clients at `https://<your-host>/v1` with model `auto`. Any OpenAI- or Anthropic-compatible client works. `/connect` has per-client recipes.

### Console accounts

Three ways into the console, in the order you should use them:

- **Named local accounts.** The first one is seeded from `ROUTER_ADMIN_USER` / `ROUTER_ADMIN_PASSWORD` while the user table is empty. After that, manage them under **Access → Console users**, and change your own password on **Your profile** (`/me`). Every console action is audited under the person's name.
- **The company account** (next section). Members of the admin groups get the console; everyone else gets the chat.
- **Break-glass.** `ROUTER_DASHBOARD_PASSWORD` opens the login page with any username, so an operator is never locked out. The audit log marks such a session as *break-glass*. Set it to a long passphrase and keep it for emergencies.

`GET /router/whoami` tells a script who it is signed in as.

### Let employees sign in with the company account

Sluss keeps no accounts for employees: they sign in with the company account. Connect it to your identity provider over OpenID Connect, and the login page gets a **Sign in with …** button. Employees land in the chat, identified by email and department. Members of your admin groups get the console. Leaving the company means leaving Sluss, because the IdP decides.

1. Register a web application at the IdP (Entra ID: *App registrations*; Okta: *Applications*; Keycloak: *Clients*) with the redirect URI `https://<your-host>/router/oidc/callback`. Enable the ID token, and make sure groups are included in it (Entra: *Token configuration → Add groups claim*; Okta: add the `groups` scope; Keycloak: a group membership mapper).
2. Set `ROUTER_OIDC_ISSUER`, `ROUTER_OIDC_CLIENT_ID` and, unless you use a public client, `ROUTER_OIDC_CLIENT_SECRET`. Set `ROUTER_PUBLIC_URL` to the same host as the redirect URI.
3. Map groups: `ROUTER_OIDC_ADMIN_GROUPS` for the console, `ROUTER_OIDC_DEPARTMENTS` as `group=department` pairs so chat traffic is logged under the right department.

With Entra ID and Intune, a Conditional Access policy on the Sluss app can require a compliant, managed device. Sluss then never has to check the device itself.

### Make the chat the company's front door

Employees who sign in land in the chat. It attaches files (text and code are read in the browser and classified as document text; images are flagged as attachments policy decides on) and, with `ROUTER_SEARCH_URL` set to a SearXNG-compatible endpoint, searches the web before answering. Self-host SearXNG next to Sluss so a search query never becomes someone else's data. A prompt that carries personal or confidential data is never sent to the search engine; the chat says so.

**Setting up SearXNG.** [`deploy/searxng/settings.yml`](../../deploy/searxng/settings.yml) is a ready configuration: JSON output on (SearXNG ships with it off, and Sluss then gets HTTP 403), no rate limiter, no public use.

- *Docker Compose:* `docker compose --profile search up -d` with `SEARXNG_SECRET` set, and `ROUTER_SEARCH_URL=http://searxng:8080/search` on the router.
- *EasyPanel:* add an App service `searxng` from the image `searxng/searxng:latest`, without a domain. Add a file mount at `/etc/searxng/settings.yml` with the file's content, and the environment variable `SEARXNG_SECRET` with a long random value. On the Sluss service, set `ROUTER_SEARCH_URL=http://<project>_searxng:8080/search`, where `<project>` is the EasyPanel project name, and redeploy.
- *Check:* in the chat, turn on 🌐 and ask something harmless. The answer shows *web search · N sources*. *search unavailable* means Sluss cannot reach SearXNG, or JSON output is off.

### Roll it out with your MDM

**Rollout** in the console (`/router/rollout`) generates what Intune, Jamf or Fleet runs: scripts that point every tool with a base URL at Sluss, a deny list of direct generative-AI domains for your DNS filter or web gateway, a block page on this instance to point the sinkhole at, a web clip for the company chat, and a daily coverage check that reports per device whether a provider can still be reached directly. The dashboard shows the result.

The full list of settings, with the issuer formats for each IdP, is in [`deploy/example.env`](../../deploy/example.env).

## 3. Put TLS in front

The container speaks plain HTTP on port 8080, so terminate TLS in a reverse proxy. Here is a minimal Caddy example, which fetches certificates automatically:

```caddy
sluss.example.eu {
    reverse_proxy 127.0.0.1:8080
}
```

nginx, Traefik or a Cloudflare Tunnel work the same way. Once the proxy is up, set `ROUTER_PUBLIC_URL=https://sluss.example.eu` so that shareable links are absolute.

## 4. Upgrade

```bash
docker pull ghcr.io/magnusfroste/sluss-eu:<tag>
docker rm -f sluss
docker run … ghcr.io/magnusfroste/sluss-eu:<tag>   # same flags, same volume
```

You can check which build is running in three places:

- The start of the log prints a banner with the version, commit, console address and data directory.
- The console footer shows the version, linked to the Version page. A yellow dot there means a newer release is out, and the Version page lists what changed and the upgrade step.
- `curl -s http://localhost:8080/version` prints it too, with no key.

### Monitoring

`GET /metrics` serves Prometheus metrics and needs a key: a signed-in admin, an admin API key or the break-glass password opens it. For the scraper, set `ROUTER_METRICS_TOKEN` to a long random value and give Prometheus that token, so it never holds an admin key:

```yaml
scrape_configs:
  - job_name: sluss
    scheme: https
    static_configs: [{ targets: ["sluss.example.eu"] }]
    authorization:
      type: Bearer
      credentials: "<ROUTER_METRICS_TOKEN>"
```

### Browser hardening and the login brake

Every response carries the standard security headers (`Content-Security-Policy` allowing this origin only, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `Permissions-Policy`). `Strict-Transport-Security` is sent when the request arrived over TLS, which your reverse proxy signals with `X-Forwarded-Proto: https`; Caddy, nginx and Traefik set it by default.

The login form brakes password guessing: after five wrong passwords for an account within fifteen minutes, further attempts are refused for a wait that doubles each time, up to fifteen minutes; a client address gets twenty. Every wrong password and every brake is in the audit log (`console.login_failed`, `console.login_throttled`). The brake is per instance and in memory, so a restart clears it.

Schema migrations run on start, and nothing in `/data` needs manual steps.

## 5. Back up

Stop the container briefly so that the SQLite file is consistent, then archive the volume:

```bash
docker stop sluss
docker run --rm -v sluss-data:/data -v "$PWD":/backup alpine \
  tar czf /backup/sluss-$(date +%F).tgz -C /data .
docker start sluss
```

To restore, extract the archive into an empty volume and start the container with that volume. Restore drills belong in your NIS2 continuity plan.

The audit chain can also be exported from the console. An auditor verifies the export offline with `audit-verify audit-chain.jsonl`, which is built from `cmd/audit-verify`.

**Anchor the chain outside the machine.** Every hour Sluss logs an `audit anchor` line with the sequence number and hash of the last record (`ROUTER_AUDIT_ANCHOR_INTERVAL` changes the interval; `off` disables). Ship the container's log to your log service or SIEM, as you do for other services, and that copy is the proof: an auditor runs `audit-verify -anchor <seq>:<hash> audit-chain.jsonl` with any anchor from before, and an export that was rewritten after that anchor fails, even if every hash in it was recomputed. `GET /router/audit/anchor` returns the current anchor for a script that files it elsewhere.

**Close the log when it gets large.** Set `ROUTER_AUDIT_ROTATE_MB` (for example `50`) and the audit file is sealed at that size: it gets a last record, moves to `audit-archive/` in the data directory, read-only, and the log continues in a new file as the same chain. Nothing is deleted, the export is still the whole log, and the archive is backed up with the rest of the data directory. `POST /router/audit/rotate` seals by hand; `GET /router/audit/segments` lists the sealed files with their SHA-256.

## 6. Air-gapped installs

1. On a connected machine, export the image: `docker save ghcr.io/magnusfroste/sluss-eu:<tag> | gzip > sluss.tar.gz`.
2. Move the file across, then run `docker load < sluss.tar.gz` on the target.
3. Set `ROUTER_UPDATE_CHECK=off` (no version check against GitHub) and `ROUTER_PRICING_SYNC=false` (no price list from OpenRouter). With those two off, Sluss makes no outbound calls except to the providers you configure.
4. Leave `OPENROUTER_API_KEY` unset and register only local models, meaning providers tagged `local`, `on-prem` or `air-gapped`. A local server that takes no API key needs no key variable.

With fail-closed policy, a request that would need a cloud provider gets a 403 and an audit entry. It never falls back to a cloud model silently.

## Checklist

- [ ] The volume is mounted at `/data`. After a restart, `docker logs sluss` should show `durable history enabled`.
- [ ] `LOCAL_API_KEY` is strong and is replaced by per-department keys.
- [ ] TLS is terminated in front, and port 8080 is bound to `127.0.0.1` or a private network only.
- [ ] The image is pinned to a `sha-` or `v` tag, not `latest`.
- [ ] `ROUTER_ADMIN_PASSWORD` has been changed on `/me` after the first sign-in, and `ROUTER_DASHBOARD_PASSWORD` is a long passphrase kept for emergencies.
- [ ] Prometheus scrapes `/metrics` with `ROUTER_METRICS_TOKEN`, not with an admin key.
- [ ] The container's log is shipped off the machine, so the hourly `audit anchor` lines are kept where the disk's owner cannot reach them.
- [ ] A backup is scheduled and has been restored once.
