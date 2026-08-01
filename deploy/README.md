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
