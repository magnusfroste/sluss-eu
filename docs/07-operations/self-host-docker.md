# Self-host Sluss with Docker

Sluss is one container and one data volume. It needs no database server and no Redis. Everything it keeps, including tenants and keys, the model roster, console rules, request history and the hash-chained audit log, lives in a SQLite file under `/data`.

The image is public and is rebuilt on every merge to `main`:

| Tag | Use |
|---|---|
| `ghcr.io/magnusfroste/sluss-eu:latest` | Trying it out |
| `ghcr.io/magnusfroste/sluss-eu:sha-<short>` | Production: pin the exact build you reviewed |
| `ghcr.io/magnusfroste/sluss-eu:v<semver>` | Production, once releases are tagged |

## 1. Run it

```bash
docker volume create sluss-data

docker run -d --name sluss --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v sluss-data:/data \
  -e LOCAL_API_KEY="$(openssl rand -hex 24)" \
  -e ROUTER_ADMIN_USER=admin \
  -e ROUTER_ADMIN_PASSWORD='choose-a-long-passphrase' \
  -e ROUTER_POLICY_PATH=builtin:nis2-baseline \
  -e OPENROUTER_API_KEY='…' \
  ghcr.io/magnusfroste/sluss-eu:latest
```

- **`-v sluss-data:/data` is the part you must not skip.** The volume holds the audit chain. Without it, everything is lost when the container is recreated.
- **`LOCAL_API_KEY`** is the bootstrap Bearer key. Once you have created per-department keys in the console, clients should use those instead.
- **`ROUTER_ADMIN_USER` / `ROUTER_ADMIN_PASSWORD`** seed the first named console user, but only when no user exists yet.
- **`ROUTER_POLICY_PATH`** selects a built-in compliance pack: `builtin:nis2-baseline`, `builtin:dora-baseline`, `builtin:gdpr-sovereign` or `builtin:pii-local`.
- **Without a provider key,** Sluss answers through a built-in mock. That is useful for a first look, but no real model is called.

The full list of settings is in [`deploy/example.env`](../../deploy/example.env).

Prefer compose? [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) sets up the same service. Swap `build:` for the `image:` line and put your values in a `.env` next to it.

## 2. Set it up

Open `http://localhost:8080/router/setup` and sign in. **Quick start** walks you through four steps:

1. A cloud model.
2. A local model, for example vLLM or Ollama on your own GPU, tagged `local`.
3. One rule that keeps personal data local.
4. A dry run that shows where a prompt would go.

Point clients at `https://<your-host>/v1` with model `auto`. Any OpenAI- or Anthropic-compatible client works. `/connect` has per-client recipes.

### Let employees sign in with the company account

Sluss keeps no employee accounts. Connect it to your identity provider over OpenID Connect, and the login page gets a **Sign in with …** button. Employees land in the chat, identified by email and department. Members of your admin groups get the console. Leaving the company means leaving Sluss, because the IdP decides.

1. Register a web application at the IdP (Entra ID: *App registrations*; Okta: *Applications*; Keycloak: *Clients*) with the redirect URI `https://<your-host>/router/oidc/callback`. Enable the ID token, and make sure groups are included in it (Entra: *Token configuration → Add groups claim*; Okta: add the `groups` scope; Keycloak: a group membership mapper).
2. Set `ROUTER_OIDC_ISSUER`, `ROUTER_OIDC_CLIENT_ID` and, unless you use a public client, `ROUTER_OIDC_CLIENT_SECRET`. Set `ROUTER_PUBLIC_URL` to the same host as the redirect URI.
3. Map groups: `ROUTER_OIDC_ADMIN_GROUPS` for the console, `ROUTER_OIDC_DEPARTMENTS` as `group=department` pairs so chat traffic is logged under the right department.

With Entra ID and Intune, a Conditional Access policy on the Sluss app can require a compliant, managed device. Sluss then never has to check the device itself.

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

You can check which build is running in two places:

- The console footer shows the commit, linked to GitHub.
- `curl -s http://localhost:8080/metrics | grep sluss_build_info` prints it too.

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

## 6. Air-gapped installs

Sluss itself makes no outbound calls except to the providers you configure.

1. On a connected machine, export the image: `docker save ghcr.io/magnusfroste/sluss-eu:<tag> | gzip > sluss.tar.gz`.
2. Move the file across, then run `docker load < sluss.tar.gz` on the target.
3. Register only local models, meaning providers tagged `local`, `on-prem` or `air-gapped`.

With fail-closed policy, a request that would need a cloud provider gets a 403 and an audit entry. It never falls back to a cloud model silently.

## Checklist

- [ ] The volume is mounted at `/data`. After a restart, `docker logs sluss` should show `durable history enabled`.
- [ ] `LOCAL_API_KEY` is strong and is replaced by per-department keys.
- [ ] TLS is terminated in front, and port 8080 is bound to `127.0.0.1` or a private network only.
- [ ] The image is pinned to a `sha-` or `v` tag, not `latest`.
- [ ] A backup is scheduled and has been restored once.
