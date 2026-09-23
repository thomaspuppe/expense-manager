# Agent instructions

- All tests MUST pass (`go test ./...`) before committing anything. If a change
  makes an existing test stale or wrong (e.g. an assertion tied to a count or
  fixed set that the change updates), fix that test in the same commit as the
  change that invalidated it — never leave it broken for a later commit to
  discover.
