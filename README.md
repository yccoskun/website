# [yusufcancoskun.com](https://www.yusufcancoskun.com)

Source code for [yusufcancoskun.com](https://www.yusufcancoskun.com).

The site uses React, TypeScript, Tailwind CSS, and Bun on the frontend. The
backend is a Go HTTP server with SQLite. The production Go binary embeds the
built frontend.

## Requirements

- Go 1.26.5
- Bun 1.3.14 or newer

## Run locally

Start the backend:

```bash
go run ./cmd/server
```

In another terminal, start the frontend:

```bash
cd web
bun install
bun run dev
```

Open `http://localhost:5173`. The Vite development server forwards API
requests to the Go server at `127.0.0.1:9000`.

The SQLite database is created automatically at `data/website.db`.

## Admin login

Admin login is disabled unless both variables are set:

```bash
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD_HASH='your-bcrypt-hash'
go run ./cmd/server
```

Generate a bcrypt password hash with:

```bash
go run ./cmd/hashpw
```

## Test and build

```bash
go test ./...

cd web
bun run build
```

## Deployment

Pushing to `main` builds and tests the application. Deployment waits for
manual approval through the GitHub `production` environment.

Server configuration and deployment notes are in [`deploy/`](deploy/README.md).
