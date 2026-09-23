# Agent instructions

## Before committing

- All tests MUST pass (`go test ./...`) before committing anything. If a
  change makes an existing test stale or wrong (e.g. an assertion tied to a
  count or fixed set that the change updates), fix that test in the same
  commit as the change that invalidated it — never leave it broken for a
  later commit to discover.
- Run `go vet ./...` too.

## Commits

- One logical change per commit (a dependency bump, a feature, a stale-test
  fix are separate commits even in the same session).
- Conventional Commits style: `feat:`, `fix:`, `docs:`, `chore:`, `test:`,
  lowercase, imperative subject line. See `git log` for examples.

## Comments

- Explain *why*, not *what*. A comment earns its place only by carrying a
  reason the code itself can't: a rejected alternative, a subtle invariant, a
  workaround for a specific bug. If removing it wouldn't confuse a future
  reader, don't write it.

## Dependencies

- Minimal by design: `modernc.org/sqlite` is the only direct dependency
  (chosen specifically because it's pure Go — no cgo, one static binary).
  Everything else in `go.mod` is transitive.
- Reach for the standard library first. Don't add a framework, ORM, or new
  direct dependency without raising it with the user first — it's a
  deliberate choice on this project, not a default.

## Decisions and plans

- Comments sometimes cite codes like `KTD1`, `KD4`, `AE2`, `R15` — these
  trace back to `docs/plans/2026-09-01-…-plan.md` (Key Technical Decisions,
  Key Decisions, Acceptance Examples, Requirements). That plan is locked
  history for the v1 build, not a running log — don't add new codes to it or
  retrofit it for unrelated later work.
- For a small change, just leave a plain comment explaining the reasoning.
- For a large new feature that deserves its own upfront design, write a new
  dated plan under `docs/plans/` (matching the existing file's shape),
  rather than editing the old one.

## PWA shell cache

- `web/assets/sw.js` precaches the app shell by listing files in `SHELL` and
  keying the cache with the `CACHE` constant (e.g. `em-shell-v4`). Whenever
  you add, remove, or rename a precached asset, bump `CACHE` — otherwise an
  already-installed PWA can keep serving a stale shell offline.
