---
title: Deployment, Usage, and Password Access
type: reference
date: 2026-09-04
topic: personal-expense-tracker
---

# Deployment, Usage, and Password Access

How to get Expense Manager onto a VPS, how to use it day to day, and exactly how
the password / token access works.

The whole app is **one static binary + one SQLite file**, run behind Caddy for
HTTPS. The deploy kit lives in [`deploy/`](../deploy).

---

## Deploying to the VPS

### Prerequisites (once)

- **DNS:** an `A` record for your domain (e.g. `expenses.example.com`) pointing
  at the VPS.
- **Caddy** installed and running on the VPS. It obtains and renews the
  Let's Encrypt certificate automatically.
- **Go 1.25+** on your *local* machine to build. Nothing is needed at runtime on
  the server — the binary is fully self-contained (pure-Go SQLite, `CGO_ENABLED=0`,
  embedded tzdata).

### 1. Build locally

```sh
./build.sh          # -> dist/expensemanager  (linux/amd64, static, stripped)
```

### 2. Provision the server (once)

```sh
sudo useradd --system --home /opt/expensemanager --shell /usr/sbin/nologin expense
sudo mkdir -p /opt/expensemanager && sudo chown expense:expense /opt/expensemanager
```

### 3. Copy the binary and config up

```sh
scp dist/expensemanager vps:/opt/expensemanager/
scp deploy/expensemanager.env.example vps:/opt/expensemanager/expensemanager.env
scp deploy/expense.service deploy/Caddyfile.example vps:~/
```

The service unit and Caddy snippet aren't needed by the running app — they're
config for the *host* (systemd, Caddy) — so they land in the home directory,
not under `/opt/expensemanager`.

Then edit `/opt/expensemanager/expensemanager.env` on the server with real
values:

```sh
EXPENSE_ADDR=127.0.0.1:8080
EXPENSE_DB=/opt/expensemanager/expenses.db
EXPENSE_PASSWORD=<your chosen login password>
EXPENSE_BEARER_TOKEN=<openssl rand -hex 32>
SESSION_SECRET=<openssl rand -hex 32>
```

- `SESSION_SECRET` must stay **stable** across restarts and redeploys. If it
  changes, every existing session cookie becomes invalid and all clients
  (including installed PWAs) are logged out.
- The app **refuses to start** if `EXPENSE_PASSWORD`, `EXPENSE_BEARER_TOKEN`, or
  `SESSION_SECRET` is missing (`internal/config/config.go`), so it never comes up
  with an open door or a per-boot-random signing key.

Lock the file down — it holds secrets:

```sh
sudo chown expense:expense /opt/expensemanager/expensemanager.env
sudo chmod 600 /opt/expensemanager/expensemanager.env
```

### 4. Install the systemd service

```sh
sudo cp ~/expense.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now expense
sudo systemctl status expense          # confirm it is running
journalctl -u expense -f               # follow logs if not
```

The unit (`deploy/expense.service` in the repo) runs as the unprivileged `expense` user,
loads the env file, and is sandboxed: `NoNewPrivileges`, `ProtectSystem=strict`,
`ProtectHome`, `PrivateTmp`, and only `/opt/expensemanager` is writable.

### 5. Wire up Caddy

Add the block from `~/Caddyfile.example` (copied up in step 3) to your
Caddyfile, with your real domain:

```
expenses.thomaspuppe.de {
	reverse_proxy 127.0.0.1:8080
}
```

Reload Caddy:

```sh
sudo systemctl reload caddy
```

The Go process only listens on `127.0.0.1:8080`, so it is never exposed
directly — all outside traffic comes through Caddy over HTTPS. Caddy sets
`X-Forwarded-Proto: https`, which the app uses to decide when to mark the
session cookie `Secure`.

### Redeploys

```sh
./build.sh
scp dist/expensemanager vps:/opt/expensemanager/
ssh vps 'sudo systemctl restart expense'
```

Installed PWAs pick up the new version on their next online load: the service
worker serves the app shell network-first and only falls back to cache when
offline, so a deploy is never pinned behind a stale cache.

### Backup

The database is a single file:

```sh
scp vps:/opt/expensemanager/expenses.db ./backups/expenses-$(date +%F).db
```

A cron job on the VPS doing `cp expenses.db` to a dated path (or an `rsync` /
object-storage push) is enough. SQLite is safe to copy while the app runs, but
copying during a write can catch an inconsistent moment — for a personal,
low-write tool that risk is negligible; if it matters, use
`sqlite3 expenses.db ".backup '/path/backup.db'"`.

---

## Using the app

1. Open `https://expenses.example.com` on your phone. You get the **login page** —
   a single password field.
2. Enter `EXPENSE_PASSWORD`. On success you land on the **numpad entry screen**.
3. **Log an expense:** type an amount, tap one of the ~8 fixed categories. It
   saves immediately. Money out only — there is no income or transfer concept.
4. **Review:** open `/review` for the month's total, the per-category breakdown,
   the transaction history, and edit/delete. Prev/next buttons move between
   months.
5. **Install as an app:** from the browser menu, "Add to Home Screen". It is a
   PWA (`web/assets/manifest.webmanifest`), so it opens full-screen like a native
   app. The session lasts 30 days, so you rarely re-enter the password.

Amounts are integer euro cents; dates are `YYYY-MM-DD` in Europe/Berlin.

For the JSON API surface every client shares, see the table in the
[README](../README.md#json-api).

---

## How the password access works

There are **two credentials**, for two kinds of client. Both are checked in
`internal/auth/auth.go`.

### 1. `EXPENSE_PASSWORD` — browser / PWA login

- The `/login` form posts the password. It is compared in constant time
  (`crypto/subtle`) against `EXPENSE_PASSWORD`.
- On success the server sets an **`em_session` cookie**. Its value is
  `<issued-unix-timestamp>.<HMAC-SHA256(timestamp, SESSION_SECRET)>`. There is
  **no server-side session store** — the cookie verifies itself via the HMAC, and
  expiry is derived from the embedded timestamp.
- Cookie flags: `HttpOnly`, `SameSite=Lax`, `MaxAge = 30 days`, and `Secure`
  whenever the request arrived over HTTPS (detected via `r.TLS` or Caddy's
  `X-Forwarded-Proto: https` header, so local plain-HTTP dev still works).
- Every browser route (`/`, `/review`) is wrapped in `GateBrowser`, which
  redirects to `/login` when the cookie is absent or its HMAC / expiry does not
  check out.
- `POST /logout` clears the cookie.

**Brute-force protection (`loginThrottle`):** every failed login is serialized
behind a mutex and then sleeps for `failureCount × 250ms`, capped at **3s**,
*while still holding the lock*. So wrong guesses cannot be run in parallel or
faster than the escalating backoff. After **15 minutes** with no failed attempt
the counter resets (the owner mistyping once is not treated as an attack). There
is deliberately **no lockout** — the correct password always works, it may just
wait a moment first.

The commented-out `rate_limit` block in `~/Caddyfile.example` on the VPS adds an
optional per-IP cap on `/login` on top of this; it needs the `caddy-ratelimit`
plugin, so it ships disabled for a stock Caddy build.

### 2. `EXPENSE_BEARER_TOKEN` — non-browser API clients

- Applies to `/api/*` only. Send `Authorization: Bearer <EXPENSE_BEARER_TOKEN>`
  (constant-time compared).
- `GateAPI` accepts **either** a valid session cookie **or** a matching bearer
  token; otherwise it returns `401 {"error":"unauthorized"}`.
- This is the path a future CSV importer, script, or MCP/AI client would use.
  There is no throttle on the token because it is a 256-bit random secret, not a
  guessable password — so keep it secret and rotate it if it leaks.

Example:

```sh
curl -H "Authorization: Bearer $EXPENSE_BEARER_TOKEN" \
  https://expenses.example.com/api/summary?month=2026-09
```

### Rotating credentials

- **Password:** change `EXPENSE_PASSWORD` in the env file and
  `sudo systemctl restart expense`. Existing sessions stay valid (they are signed
  with `SESSION_SECRET`, not the password).
- **Bearer token:** change `EXPENSE_BEARER_TOKEN` and restart. Update any scripts.
- **Session secret:** change `SESSION_SECRET` and restart. This invalidates all
  sessions — every browser and PWA must log in again. Do this if you believe a
  session cookie or the secret itself has leaked.
