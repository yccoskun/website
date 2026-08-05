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

## Edge TLS / HSTS

**Owner:** Cloudflare (SSL/TLS → Edge Certificates) — not Go, not Caddy.

| Requirement | Policy |
|-------------|--------|
| **HSTS** | `Strict-Transport-Security` with `max-age` ≥ `15552000` (180 days). Preload is optional / not required. |
| **Always Use HTTPS** | HTTP must redirect with a 3xx and `Location: https://…` for apex and www (typically 301/302; 307/308 also fine). |

Dashboard: enable **Always Use HTTPS**, then enable **HSTS** with `max-age` ≥ 15552000. `includeSubDomains` is optional: the zone also serves `ssh.yusufcancoskun.com` (tunnel to sshd). HSTS does not affect SSH itself, but browsers would force HTTPS for any *web* clients on that hostname — prefer leaving `includeSubDomains` off unless you understand that scope. The smoke script does **not** assert `includeSubDomains`.

**Non-goals:** no HSTS in Go; no HSTS on plain-HTTP localhost; do not add conflicting HSTS to the loopback Caddyfile.

**Mixed content:** the admin SPA uses same-origin embedded assets (`/assets/*`); CSP already restricts to `'self'`. The smoke script only checks quoted `src=` / `href=` for absolute `http://` in `/admin` HTML.

`deploy/smoke-edge-tls.sh` will fail until these Cloudflare edge settings are applied. See [Smoke checks after deploy](#smoke-checks-after-deploy).

## Rate limiting

Ownership by hop:

| Hop | Role |
|-----|------|
| **Cloudflare** | Edge / WAF / volumetric rate limiting (preferred first line of defense). |
| **Go app** | Login-attempt throttle on `POST /api/admin/login`: ~10 attempts / 15 minutes **per real client IP**. When the peer matches `TRUSTED_PROXIES`, the app keys on a valid `CF-Connecting-IP`; otherwise it uses `RemoteAddr`. Failed and successful attempts both count. A trusted peer with a missing/invalid `CF-Connecting-IP` gets **400** and is not counted. |
| **Caddy** | Strips spoofable client IP headers (`X-Forwarded-For`, `X-Real-IP`). `CF-Connecting-IP` reaches Go by **passthrough** from cloudflared (default `reverse_proxy` behavior) — do not delete-and-readd it with `header_up` placeholders (empty CF → Go `400 missing client ip` when loopback is trusted). Does **not** own rate limits. |

Do **not** trust raw `X-Forwarded-For` from the public internet without a trusted hop. The app never keys the login limiter on `X-Forwarded-For` and never reads that header for client IP.

Trust boundary: only peers on the explicit `TRUSTED_PROXIES` allowlist may present `CF-Connecting-IP`. Empty `TRUSTED_PROXIES` (the default) means **no** proxy trust — loopback peers are not special. Same-host processes can no longer spoof client IP via that header unless you deliberately allowlist them.

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

Alert lines only (`alert=1`):

```bash
sudo journalctl -u website --since today | grep 'alert=1'
sudo journalctl -u website -f | grep --line-buffered 'alert=1'
```

Filter by event name:

```bash
sudo journalctl -u website --since today | grep 'security event=login_failure'
sudo journalctl -u website --since today | grep 'security event=rate_limit'
sudo journalctl -u website --since today | grep 'security event=login_success'
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
security event=<name> ip=<ip> route=<path> [key=value ...] [alert=1 reason=<reason> ...]
```

| Event | When emitted | Extra fields |
|-------|----------------|--------------|
| `login_failure` | Failed admin login | `route`; `alert=1 reason=login_failure_burst` when threshold hit |
| `login_success` | Successful admin login (after session create) | `route`; `alert=1 reason=login_success_after_failures failures=N` when ≥5 prior failures |
| `rate_limit` | Login throttle hit (429) | `route`; `alert=1 reason=rate_limit` once per IP / 15m |
| `session_binding_mismatch` | Soft session binding failed (UA or IP /24|/48 hash) | `route` |
| `export` | Successful content export | `route`; always `alert=1 reason=export` |
| `import` | Successful content import | `route`, counts, `replace_*`; `alert=1 reason=import_replace` only if any `replace_*=true` |
| `media_upload` | Successful media upload | `route`; `alert=1 reason=media_upload_spike count=N` when threshold hit |
| `media_delete` | Media item deleted | `route`, `id` |

Routine single login failure, one upload, non-replace import, or successful login with
fewer than 5 prior failures: **base log only, no `alert=`**. Intentional export always
alerts (high-signal audit); treat that as expected when you run backups yourself.

Example:

```
security event=import ip=203.0.113.5 route=/api/admin/import settings_upserted=4 pages_upserted=12 work_created=3 studio_created=2 sections_created=5 entries_created=18 replace_work=true replace_studio=true replace_resume=true alert=1 reason=import_replace
```

Logs **never** include usernames, passwords, session tokens, or full export/import
payloads — only event metadata and counts.

### Alert thresholds

In-process thresholds (single-tenant, low noise). When a threshold fires, `alert=1` and
`reason=` are attached to the **same** base event line (once per IP/reason window unless noted).

| Signal | Base event | When `alert=1` |
|--------|------------|----------------|
| Repeated login 429 | `rate_limit` | First 429 per IP / 15m (`reason=rate_limit`); further 429s in the window log without re-alerting |
| Burst failed logins | `login_failure` | ≥5 failures / 15m / IP → once per window (`reason=login_failure_burst`) |
| Success after failures | `login_success` | ≥5 prior failures in the same failure window → once (`reason=login_success_after_failures`, `failures=N`) |
| Export / import replace | `export` / `import` | `export`: always (`reason=export`); `import`: only if any `replace_*=true` (`reason=import_replace`) |
| Media upload spikes | `media_upload` | ≥15 uploads / 10m / IP → once (`reason=media_upload_spike`, `count=N`) |

### Alert watcher (ops path)

`deploy/security-alert-watch.sh` tails the systemd unit journal (default `UNIT=website`) and
prints lines containing `alert=1`. If `WEBHOOK_URL` is set, each matching line is POSTed as
`text/plain` (no secrets beyond what is already in the log line). Webhook failures are
logged to stderr so a dead endpoint is visible.

```bash
# Follow alerts on the server (foreground)
sudo ./deploy/security-alert-watch.sh

# Custom unit name
sudo UNIT=website ./deploy/security-alert-watch.sh

# Optional webhook (Slack incoming webhook, ntfy, etc.)
sudo WEBHOOK_URL='https://example.invalid/hook' ./deploy/security-alert-watch.sh
```

Manual smoke check (confirm the watcher and app path once after deploy):

```bash
# Terminal A: start the watcher
sudo ./deploy/security-alert-watch.sh

# Terminal B: trigger one rate_limit alert (11+ failed logins from the same IP),
# or inspect recent alerts:
sudo journalctl -u website --since "7 days ago" | grep 'alert=1' | head
```

Keep the watcher in `screen`/`tmux`, or wrap it in a simple user systemd unit if you want
it always on. This is the documented minimal alerting path (not a full APM).

### Cloudflare (optional edge complement)

Cloudflare WAF / rate-limit notifications are an **optional edge** complement. They do not
replace app-level `alert=1` lines: edge signals are about HTTP at the proxy, while these
journal alerts are about authenticated admin actions and login thresholds inside the Go app.

Content import JSON is **trusted admin input**: only authenticated admins who complete
the password step-up (T7) may submit it. Treat dumps like credentials — do not paste
untrusted third-party JSON. Residual XSS/injection risk in stored CMS fields is
accepted for the single operator. Destructive list replace requires both dump
`replace_work` / `replace_studio` / `replace_resume` **and** matching request
`confirm_replace_*` flags; password step-up remains required for every import.

### What to alert or investigate

| Signal | Likely cause | Action |
|--------|----------------|--------|
| `alert=1 reason=rate_limit` | Login brute force or 429 storm | Check Cloudflare/WAF; consider blocking IP at edge; review whether admin login should stay exposed |
| `alert=1 reason=login_failure_burst` | Wrong password attempts or credential stuffing | Correlate IPs; confirm no legitimate lockout; tighten edge rules if sustained |
| `alert=1 reason=login_success_after_failures` | Possible credential stuffing then success | Confirm it was you; rotate password if unexpected |
| `alert=1 reason=export` / `import_replace` | Content dump or destructive import | Confirm you (or CI) did it; rotate credentials if unexpected |
| `alert=1 reason=media_upload_spike` | Bulk upload or compromise | Confirm admin activity; review media library |
| `session_binding_mismatch` | Session cookie reused from a different UA or IP prefix | Expected after network/browser changes when binding is on; re-login only — not a lockout. Investigate if unexpected / frequent from unfamiliar IPs |
| `media_delete` outside known admin activity | Possible compromise or unexpected automation | Confirm you did not run it; rotate `ADMIN_PASSWORD_HASH`; review session/access |

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
set. Write `/etc/website.env` (mode `0640`, owned `root:deploy`). A full
annotated template is in [`website.env.example`](website.env.example):

```bash
sudo cp deploy/website.env.example /etc/website.env
# edit secrets, then chown root:deploy and chmod 0640
```

Minimal production example:

```bash
ADMIN_USERNAME=admin
ADMIN_PASSWORD_HASH=$2a$12$...   # prefer: go run ./cmd/hashpw (interactive); automation: go run ./cmd/hashpw < /path/to/secret (mode 0600); argv rejected — avoid inline secrets in shell commands
# Required for Caddy → Go on loopback so login rate limits / session binding
# key on CF-Connecting-IP (empty = no proxy trust; all clients share 127.0.0.1).
TRUSTED_PROXIES=127.0.0.0/8,::1/128
# Required in production; match unit Environment= (env file overrides unit).
SESSION_BINDING=1
```

`ADDR` (default `127.0.0.1:9000`), `DB_PATH` (default `data/website.db`), and
`SITE_URL` (default `https://www.yusufcancoskun.com`) all have correct defaults
for this server and can be omitted. Do **not** omit `TRUSTED_PROXIES` in
production behind Caddy — see below.

### Trusted proxies (`TRUSTED_PROXIES`)

Comma-separated allowlist of peers permitted to present `CF-Connecting-IP`.
Empty (default) means **no** proxy trust: the app always uses `RemoteAddr` and
ignores `CF-Connecting-IP`. Invalid entries cause the process to refuse to
start.

Tokens:

| Token | Meaning |
|-------|---------|
| CIDR | e.g. `127.0.0.1/32`, `127.0.0.0/8`, `::1/128` |
| Bare IP | Normalized to `/32` (IPv4) or `/128` (IPv6) |
| `unix` | Trust Unix-domain peers (`RemoteAddr` `@…` or a socket path) |

Typical Caddy → Go over loopback TCP:

```bash
TRUSTED_PROXIES=127.0.0.0/8,::1/128
```

`X-Forwarded-For` is never trusted, regardless of this allowlist. Caddy must
passthrough `CF-Connecting-IP` from cloudflared (see `deploy/Caddyfile`); do
not strip-and-reset that header with placeholders.

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

### Soft session binding (required in production)

**Production:** required. The reference `website.service` sets
`Environment=SESSION_BINDING=1`. Per systemd.exec, `EnvironmentFile=`
overrides `Environment=`: the unit value applies when `/etc/website.env`
omits the key, but a falsy `SESSION_BINDING` in the env file defeats the
unit. Put `SESSION_BINDING=1` in `/etc/website.env` (or omit the key so
the unit value applies); never leave a falsy value in production. Local
development may leave it unset (off by default).

Truthy values: `1`, `true`, `yes` (case-insensitive). Restart the unit after
changing the file.

```bash
SESSION_BINDING=1
```

Binding uses `ClientIP` for the IP prefix hash (IPv4 `/24` or IPv6 `/48`)
and the request User-Agent for the UA hash. Production behind Caddy **must**
set `TRUSTED_PROXIES` (already required) so binding keys on
`CF-Connecting-IP`. With an empty allowlist (no proxy trust), `ClientIP` is
`RemoteAddr` — behind Caddy on loopback, sessions bind to the shared
loopback peer. With `TRUSTED_PROXIES` set, binding uses `CF-Connecting-IP`;
trusted peers with a missing or invalid `CF-Connecting-IP` fail closed at
login (empty `ClientIP`).

When enabled, login stores SHA-256 hex of the UA and of the IP prefix — never
raw UA/IP. Login fails closed if either hint is missing (empty User-Agent or
unparseable client IP). Authenticated admin requests (including protected
`/media/{id}`) that present a session with **both** hashes set must match
both; **either** mismatch destroys the session, clears `__Host-session`, and
returns `401` with `reauth_required` (and logs `session_binding_mismatch`).
There is **no** account lockout — only re-login.

With the flag off, login does not store binding hashes (NULL columns). Sessions
created while the flag was off (or before this feature) keep expiry-only
validation even after you enable the flag, until the next login. After
enabling in production, **re-login** so new sessions store binding hashes.

**False-positive risks:** CGNAT `/24` churn, mobile carrier IP handoffs, and
privacy browsers that rotate User-Agent can force an unexpected re-login.
Accept that friction for the single-operator admin surface in production.

## Cryptographic posture

Acceptance checklist before shipping any change that touches auth, sessions, or
cookies (see `internal/auth/`, `internal/services/sessions.go`):

- [ ] **Passwords** — stored only as bcrypt hashes via `ADMIN_PASSWORD_HASH`
      (prefer interactive `go run ./cmd/hashpw`; for automation redirect from a
      mode-0600 file, e.g. `go run ./cmd/hashpw < /path/to/secret`. Argv is
      rejected; avoid inline secrets in shell commands — they can still land in
      history/process argv). Never plaintext, never a reversible encoding,
      never logged.
- [ ] **Sessions** — raw tokens are 256-bit (`crypto/rand`, 32 bytes → 64 hex
      chars). The database stores only the SHA-256 hash of the token
      (`token_hash` column); the raw token exists solely in the session
      cookie sent to the browser.
- [ ] **Cookie flags** — name `__Host-session` with `HttpOnly`, `SameSite=Lax`,
      `Secure` always, `Path=/`, and no `Domain` attribute (OWASP `__Host-`
      requirements). Legacy `session` is cleared on login/logout and is never
      read for authentication. After deploy, browsers still holding only the
      old `session` cookie get 401 until re-login (login clears the legacy name).
      Locally, open the Vite app as `http://localhost` (not a LAN IP): browsers
      treat `localhost` as a secure context, so `Secure`/`__Host-` works over
      plain HTTP; LAN IPs over HTTP will not store the cookie. Public traffic
      always arrives over HTTPS via Cloudflare/tunnel.
- [ ] **Auth model** — sessions are opaque, server-side, and validated by
      hash lookup. No JWT or other self-contained/stateless token format.
- [ ] **Env file** — `/etc/website.env` stays mode `0640`, owned
      `root:deploy` (see [Environment file](#environment-file) above).
- [ ] **Trusted proxies** — production behind Caddy sets
      `TRUSTED_PROXIES` (e.g. `127.0.0.0/8,::1/128`) so login rate limits and
      session binding key on `CF-Connecting-IP`, not a shared loopback peer.
- [ ] **Session binding** — production: unit always sets
      `Environment=SESSION_BINDING=1`; env file should match `1` (or omit
      the key) so it does not override off. After enabling, re-login so
      sessions store UA/IP prefix hashes (older NULL-hash sessions stay
      expiry-only until then).

### Auth trust boundary

- React `RequireAdmin` is UX-only (hide the admin shell / show login). It does
  not authorize access.
- Authenticated admin routes (`RequireSession`) are the enforcement point
  (session cookie + Sec-Fetch-Site). Login and logout are not covered by
  `RequireSession`.
- Authenticated admin JSON responses set `Cache-Control: private, no-store`.

### Sec-Fetch-Site on authenticated admin APIs

Authenticated admin routes (`RequireSession`) allow a missing
`Sec-Fetch-Site` so curl, tests, and scripts keep working. Browsers always
send the header. Allowed values: missing, `same-origin`, `same-site`,
`none`. `cross-site` and any other non-allowlisted value → 403. SPA
same-origin fetches with `credentials: "include"` send `same-origin` and
are unaffected. Login and logout are not covered by this check.

### CORS: deny (same-origin admin API)

- Credentialed admin API is same-origin only (SPA relative fetches +
  `__Host-session` cookie).
- Do not add `Access-Control-Allow-Origin: *` (or any permissive CORS) on Go
  or Caddy.
- Absence of CORS headers is intentional; browsers will not expose
  cross-origin responses to JS.
- If CORS is ever required: explicit origin allowlist, never `*`, and a
  threat review of `Access-Control-Allow-Credentials` with cookie auth
  (OWASP A01/A02).

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

Optional production-only edge TLS / HSTS check (not CI; probes live Cloudflare):

```bash
./deploy/smoke-edge-tls.sh
# or: bash deploy/smoke-edge-tls.sh
```

Asserts HSTS (`max-age` ≥ 15552000) on HTTPS for apex and www, HTTP→HTTPS redirects (3xx + `Location` to apex/www over https) for both hostnames, and no absolute `http://` in quoted `src`/`href` on `/admin` HTML. Fails until Cloudflare Always Use HTTPS + HSTS are configured (see [Edge TLS / HSTS](#edge-tls--hsts)).
