# Deploy notes — yusufcancoskun.com

Reference docs for the existing Hetzner server. Nothing in this directory is
executed by CI; `Caddyfile` and `website.service` mirror what is already
installed on the server.

## Architecture

```
Browser ──HTTPS──> Cloudflare ──tunnel──> cloudflared ──> Caddy 127.0.0.1:8080 ──> Go 127.0.0.1:9000
```

- TLS terminates at Cloudflare; nothing on the server listens publicly.
- `cloudflared` ingress routes `www.yusufcancoskun.com` and the apex to Caddy,
  and `ssh.yusufcancoskun.com` to sshd (SSH also goes through the tunnel).
- Backups are Hetzner server snapshots — no backup scripts or timers in the app.

## Rate limiting

Ownership by hop:

| Hop | Role |
|-----|------|
| **Cloudflare** | Edge / WAF / volumetric rate limiting (preferred first line of defense). |
| **Go app** | Login-attempt throttle on `POST /api/admin/login`: ~10 attempts / 15 minutes **per real client IP**. When the peer is loopback (Caddy), the app keys on `CF-Connecting-IP`; otherwise it uses `RemoteAddr`. Failed and successful attempts both count. |
| **Caddy** | Strips spoofable client IP headers (`X-Forwarded-For`, `X-Real-IP`) and forwards Cloudflare’s `CF-Connecting-IP`. Does **not** own rate limits. |

Do **not** trust raw `X-Forwarded-For` from the public internet without a trusted hop. The app never keys the login limiter on `X-Forwarded-For`.

Trust boundary: the Go process binds loopback only, so the only peers that can present `CF-Connecting-IP` are local proxies (Caddy). A process on the same host could spoof that header; that is accepted for this single-tenant deploy.

## Security logging

The app writes security events via Go's default `log` package to **stderr**; systemd
captures them in **journald** under the `website` unit (`deploy/website.service` →
`/etc/systemd/system/website.service`).

### Viewing logs

Follow live:

```bash
sudo journalctl -u website -f
```

Recent window:

```bash
sudo journalctl -u website --since "1 hour ago"
sudo journalctl -u website --since today
```

Security events only (structured prefix `security event=`):

```bash
sudo journalctl -u website --since "24 hours ago" | grep 'security event='
sudo journalctl -u website -f | grep --line-buffered 'security event='
```

Filter by event name:

```bash
sudo journalctl -u website --since today | grep 'security event=login_failure'
sudo journalctl -u website --since today | grep 'security event=rate_limit'
```

Count `rate_limit` hits per IP (spot repeat offenders):

```bash
sudo journalctl -u website --since "24 hours ago" \
  | grep 'security event=rate_limit' \
  | sed -n 's/.* ip=\([^ ]*\).*/\1/p' \
  | sort | uniq -c | sort -rn | head
```

### Log format

Each line is a single space-separated record:

```
security event=<name> ip=<ip> [<key>=<value> ...]
```

| Event | When emitted | Extra fields |
|-------|----------------|--------------|
| `login_failure` | Failed admin login | — |
| `rate_limit` | Login throttle hit (429) | — |
| `export` | Successful content export | — |
| `import` | Successful content import | `pages_upserted`, `work_created` |
| `media_delete` | Media item deleted | `id` |

Successful admin login is **not** logged — there is no `login_success` event.

Example:

```
security event=import ip=203.0.113.5 pages_upserted=12 work_created=3
```

Logs **never** include usernames, passwords, session tokens, or full export/import
payloads — only event metadata and counts.

### What to alert or investigate

| Signal | Likely cause | Action |
|--------|----------------|--------|
| Many `rate_limit` lines from one `ip=` | Login brute force or 429 storm | Check Cloudflare/WAF; consider blocking IP at edge; review whether admin login should stay exposed |
| Burst of `login_failure` | Wrong password attempts or credential stuffing | Correlate IPs; confirm no legitimate lockout; tighten edge rules if sustained |
| `export`, `import`, or `media_delete` outside known admin activity | Possible compromise or unexpected automation | Confirm you (or CI) did not run it; rotate `ADMIN_PASSWORD_HASH`; review session/access |

For a single-tenant site, a few failed logins are normal noise; sustained clusters from
unfamiliar IPs or off-hours bulk `export`/`import` deserve a closer look.

## Layout on the server

| Path | Purpose |
|------|---------|
| `/opt/website/website` | Static Go binary (replaced by `deploy-website`) |
| `/opt/website/data/website.db` | SQLite DB + WAL (default `DB_PATH` is `data/website.db`, relative to the unit's `WorkingDirectory`) |
| `/etc/website.env` | Runtime environment (optional but needed for admin login) |
| `/etc/systemd/system/website.service` | App unit (already installed; see reference copy here) |
| `/etc/caddy/Caddyfile` | Reverse proxy (already installed; see reference copy here) |
| `/etc/cloudflared/config.yml` | Tunnel ingress (managed outside this repo) |

## Environment file

Admin login is disabled until `ADMIN_USERNAME` and `ADMIN_PASSWORD_HASH` are
set. Write `/etc/website.env` (mode `0640`, owned `root:deploy`):

```bash
ADMIN_USERNAME=admin
ADMIN_PASSWORD_HASH=$2a$12$...   # generate with: go run ./cmd/hashpw
```

`ADDR` (default `127.0.0.1:9000`), `DB_PATH` (default `data/website.db`), and
`SITE_URL` (default `https://www.yusufcancoskun.com`) all have correct defaults
for this server and can be omitted.

The live unit needs one added line to pick this file up (already present in
this repo's reference copy):

```ini
EnvironmentFile=-/etc/website.env
```

Then `sudo systemctl daemon-reload && sudo systemctl restart website`.

## CI → deploy flow

On push to `main`, `.github/workflows/deploy.yml`:

1. **build job** (GitHub-hosted): Go tests + `govulncheck`, Bun/Vite frontend
   build, copies `web/dist` into `internal/static/dist` for `//go:embed`,
   cross-compiles a static Linux amd64 binary, uploads it as the `website`
   artifact.
2. **deploy job** (self-hosted runner, `linux` + `website` labels): waits for
   manual approval, then downloads the artifact and runs
   `sudo /usr/local/bin/deploy-website`.

The approval gate comes from the `production` environment: in the GitHub repo
go to **Settings → Environments → production → Required reviewers** and add
yourself. Every deploy then pauses until you approve it in the Actions UI.

`deploy-website` should atomically replace `/opt/website/website` and run
`systemctl restart website`.

## Smoke checks after deploy

```bash
curl -sI https://www.yusufcancoskun.com/api/health
curl -s https://www.yusufcancoskun.com/robots.txt
curl -s https://www.yusufcancoskun.com/sitemap.xml | head
systemctl status website caddy cloudflared
```
