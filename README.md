# Expense Manager

A personal, single-user expense tracker. One small Go binary serves a
numpad-first PWA and a JSON API over a local SQLite database. Money out only —
open, type an amount, tap a category, done.

- **Fast capture:** the app opens on a live numpad; type an amount, tap one of
  ~10 fixed categories, and it is saved.
- **Where did it go:** monthly total and per-category breakdown, with prev/next
  month navigation.
- **One source of truth:** SQLite on the server; phone (installable PWA),
  desktop, and any future client all read/write the same JSON API.

Built to the plan in [`docs/plans/`](docs/plans/).

## Requirements

- Go 1.25+ to build (nothing at runtime — the binary is self-contained).

## Configuration (environment)

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `EXPENSE_PASSWORD` | yes | — | the single owner login password |
| `EXPENSE_BEARER_TOKEN` | yes | — | token accepted on `/api/*` for non-browser clients |
| `SESSION_SECRET` | yes | — | HMAC key for session cookies; keep stable across restarts |
| `EXPENSE_DB` | no | `expenses.db` | SQLite file path |
| `EXPENSE_ADDR` | no | `127.0.0.1:8080` | listen address |

Generate secrets with `openssl rand -hex 32`.

## Run locally

```sh
export EXPENSE_PASSWORD=dev EXPENSE_BEARER_TOKEN=dev-token SESSION_SECRET=dev-secret
go run .
# open http://127.0.0.1:8080
```

## Test

```sh
go test ./...
go vet ./...
```

## Build for the server

```sh
./build.sh          # -> dist/expensemanager (linux/amd64, static)
```

## Deploy (IONOS VPS + Caddy)

The whole app is one binary plus a SQLite file — copy them up and run behind
Caddy, which supplies automatic HTTPS.

1. **Provision once** on the VPS:
   ```sh
   sudo useradd --system --home /opt/expensemanager --shell /usr/sbin/nologin expense
   sudo mkdir -p /opt/expensemanager && sudo chown expense:expense /opt/expensemanager
   ```
2. **Copy the binary and config:**
   ```sh
   scp dist/expensemanager vps:/opt/expensemanager/
   scp deploy/expensemanager.env.example vps:/opt/expensemanager/expensemanager.env  # then edit real secrets
   ```
3. **Install the service:**
   ```sh
   sudo cp deploy/expense.service /etc/systemd/system/
   sudo systemctl daemon-reload && sudo systemctl enable --now expense
   ```
4. **Proxy with Caddy:** add the block from `deploy/Caddyfile.example` (with your
   domain) to the Caddyfile and reload Caddy.

Redeploys are: `./build.sh`, `scp` the new binary, `sudo systemctl restart expense`.

## Backup

The database is a single file — back it up by copying it:

```sh
scp vps:/opt/expensemanager/expenses.db ./backups/expenses-$(date +%F).db
```

## Not in v1 (deferred)

CSV bank import, AI/MCP access, offline logging, category customization, and a
native iOS app — all future clients on the same API. Accounts, budgets, income,
multi-currency, and multi-user are deliberately out of scope.
