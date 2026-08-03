# Architecture

The service is a synchronous adapter around a long-lived Chrome process.

```text
HTTP request
    |
    v
strict decoder and validation
    |
    v
bounded render queue
    |
    v
isolated Chrome tab and resource policy
    |
    v
Chrome DevTools PrintToPDF
    |
    v
bounded PDF response
```

`cmd/pdf-generator` owns process lifecycle and graceful shutdown. `internal/config` converts environment variables into validated configuration. `internal/httpapi` owns the HTTP contract, authentication, concurrency limits, timeouts, metrics, and error responses. `internal/pdf` owns option validation, browser lifecycle, resource interception, asset readiness, and PDF generation.

A browser process is reused to avoid cold startup on every request. Each render gets a separate tab. Request cancellation is bridged into the tab context. The HTTP layer limits concurrent tabs and rejects requests that exceed the configured queue timeout.

Remote network access is denied by default. When exact hosts are configured, Chrome Fetch interception decides whether each resource request can continue. This is complemented by deployment-level egress controls.
