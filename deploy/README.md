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
| `import` | Successful content import | `settings_upserted`, `pages_upserted`, `work_created`, `studio_created`, `sections_created`, `entries_created`, `replace_work`, `replace_studio`, `replace_resume` |
| `media_delete` | Media item deleted | `id` |

Successful admin login is **not** logged — there is no `login_success` event.

Example:

```
security event=import ip=203.0.113.5 settings_upserted=4 pages_upserted=12 work_created=3 studio_created=2 sections_created=5 entries_created=18 replace_work=true replace_studio=true replace_resume=true
```

Logs **never** include usernames, passwords, session tokens, or full export/import
payloads — only event metadata and counts.

Content import JSON is **trusted admin input**: only authenticated admins who complete
the password step-up (T7) may submit it. Treat dumps like credentials — do not paste
untrusted third-party JSON. Residual XSS/injection risk in stored CMS fields is
accepted for the single operator. Destructive list replace requires both dump
`replace_work` / `replace_studio` / `replace_resume` **and** matching request
`confirm_replace_*` flags; password step-up remains required for every import.

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

Production always serves the **embedded** frontend build (compiled into the
binary from `internal/static/dist`). Do **not** set `STATIC_DIR` or
`ALLOW_STATIC_DIR` in `/etc/website.env` — leaving them unset is what makes
the server use the embedded build. `STATIC_DIR` is a dev-only override, gated
by `ALLOW_STATIC_DIR`, and the resolved directory must be a subdirectory of
the process working directory (`WorkingDirectory=/opt/website` on this
server) — not the working directory itself — or the server refuses to start.
Disk serving also uses a rooted filesystem open so nested symlinks cannot
escape that tree. For local development, after `bun run build` in `web/`,
run the server from the repo root with:

```bash
ALLOW_STATIC_DIR=1 STATIC_DIR=web/dist go run ./cmd/server
```

The live unit needs one added line to pick this file up (already present in
this repo's reference copy):

```ini
EnvironmentFile=-/etc/website.env
```

Then `sudo systemctl daemon-reload && sudo systemctl restart website`.

## Cryptographic posture

Acceptance checklist before shipping any change that touches auth, sessions, or
cookies (see `internal/auth/`, `internal/services/sessions.go`):

- [ ] **Passwords** — stored only as bcrypt hashes via `ADMIN_PASSWORD_HASH`
      (generated with `go run ./cmd/hashpw`). Never plaintext, never a
      reversible encoding, never logged.
- [ ] **Sessions** — raw tokens are 256-bit (`crypto/rand`, 32 bytes → 64 hex
      chars). The database stores only the SHA-256 hash of the token
      (`token_hash` column); the raw token exists solely in the session
      cookie sent to the browser.
- [ ] **Cookie flags** — `HttpOnly` always set; `SameSite=Lax` always set;
      `Secure` is set for every public Hostname, including tunnel/proxy
      hostnames (e.g. `*.trycloudflare.com`), and is only skipped for
      loopback Hosts (`localhost`, `127.0.0.1`, `::1`) used in local dev.
- [ ] **Auth model** — sessions are opaque, server-side, and validated by
      hash lookup. No JWT or other self-contained/stateless token format.
- [ ] **Env file** — `/etc/website.env` stays mode `0640`, owned
      `root:deploy` (see [Environment file](#environment-file) above).

### Sec-Fetch-Site on authenticated admin APIs

Authenticated admin routes (`RequireSession`) allow a missing
`Sec-Fetch-Site` so curl, tests, and scripts keep working. Browsers always
send the header. Allowed values: missing, `same-origin`, `same-site`,
`none`. `cross-site` and any other non-allowlisted value → 403. SPA
same-origin fetches with `credentials: "include"` send `same-origin` and
are unaffected. Login and logout are not covered by this check.

## CI → deploy flow

On push to `main`, `.github/workflows/deploy.yml` runs a two-job pipeline.
Trust model for this single-tenant setup:

### Build

Only the GitHub-hosted `build` job on this repo’s `main` produces the
`website` artifact. That job runs Go tests, pinned `govulncheck`, a frozen
Bun install, the Vite frontend build (embedded into the binary), and a
static Linux amd64 `go build`. No other workflow or host should be treated
as an authoritative producer of that artifact.

### Approval

The `deploy` job uses Environment `production` with required reviewers
(Settings → Environments → production → Required reviewers). Only those
reviewers can approve a release; every deploy pauses in the Actions UI
until approval.

### Staging path

After approval, `actions/download-artifact` on the self-hosted runner
(`self-hosted`, `linux`, `website`) writes the artifact to
`/home/github-runner/staging`. Only that runner identity should write that
path. Humans should not drop unsigned binaries there for `deploy-website`
to pick up — the intended input is always the CI-built artifact from
`main`.

### Activate

`sudo /usr/local/bin/deploy-website` (server-local, not in this repo)
atomically replaces `/opt/website/website` and restarts the unit
(`systemctl restart website`).

### Frontend audit

`.github/workflows/frontend-audit.yml` runs weekly (Mondays 06:00 UTC) and
on `workflow_dispatch`. It installs with a frozen lockfile and runs
`bun audit`. Treat red runs as actionable: bump the dependency, or document
an ignore with rationale. Local parity: `bun run audit` in `web/`.

## Smoke checks after deploy

```bash
curl -sI https://www.yusufcancoskun.com/api/health
curl -s https://www.yusufcancoskun.com/robots.txt
curl -s https://www.yusufcancoskun.com/sitemap.xml | head
systemctl status website caddy cloudflared
```
