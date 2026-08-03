# Contributing

## Development setup

Install Go 1.25 or newer and a Chromium-compatible browser.

```bash
go mod download
make check
```

Run the browser integration test explicitly:

```bash
CHROME_PATH=/usr/bin/google-chrome CHROME_NO_SANDBOX=true make integration
```

## Pull requests

Keep changes focused, add tests for changed behavior, run `make check`, and update the OpenAPI document when the HTTP contract changes.

Use conventional commit prefixes such as `feat:`, `fix:`, `test:`, `docs:`, and `refactor:`.
