# Go PDF Renderer

[![CI](https://github.com/zhenya3000/go-pdf-renderer/actions/workflows/ci.yml/badge.svg)](https://github.com/zhenya3000/go-pdf-renderer/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/zhenya3000/go-pdf-renderer.svg)](https://pkg.go.dev/github.com/zhenya3000/go-pdf-renderer)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A small, production-minded HTML-to-PDF service written in Go and backed by headless Chrome.

The project focuses on predictable rendering, bounded resource usage, strict API validation, secure defaults, and straightforward deployment.

## Features

- HTML-to-PDF rendering through the Chrome DevTools Protocol
- A4, Letter, Legal, and custom page sizes
- Landscape mode, margins, scale, backgrounds, headers, and footers
- Font and image readiness modes with an optional render delay
- Bounded request size, queue time, render time, concurrency, and PDF size
- API key authentication and explicit CORS origins
- Remote assets denied by default with an exact-host allowlist
- JavaScript disabled by default
- Health, readiness, build information, and Prometheus metrics endpoints
- Embedded Swagger UI for exploring and calling the API
- Graceful shutdown and structured JSON logs
- Unit, race, security-policy, and browser integration tests
- Docker image, Compose setup, OpenAPI 3.1 contract, and GitHub Actions CI

## Quick start

### Docker Compose

```bash
cp .env.example .env
docker compose up --build
```

### Local Go process

Install Go 1.25 or newer and Chrome or Chromium.

```bash
CHROME_PATH=/usr/bin/google-chrome go run ./cmd/go-pdf-renderer
```

If the local runtime cannot use the Chrome sandbox, set `CHROME_NO_SANDBOX=true`. Do this only inside an isolation boundary you control.

Open [http://localhost:3001/docs/](http://localhost:3001/docs/) to inspect the contract and call the API from Swagger UI. If `API_KEY` is configured, use the **Authorize** button before sending a request.

## Generate a PDF

```bash
curl --fail-with-body \
  --output report.pdf \
  --request POST http://localhost:3001/v1/pdf \
  --header 'Content-Type: application/json' \
  --header 'X-API-Key: change-me' \
  --data '{
    "html": "<!doctype html><html><head><meta charset=\"utf-8\"><style>@page { size: A4 } body { font-family: sans-serif }</style></head><body><h1>Hello, PDF</h1></body></html>",
    "filename": "report.pdf",
    "options": {
      "format": "A4",
      "print_background": true,
      "margins": {
        "top_mm": 12,
        "right_mm": 12,
        "bottom_mm": 12,
        "left_mm": 12
      }
    }
  }'
```

The compatibility endpoint `POST /generate-pdf` accepts the same request. New clients should use `POST /v1/pdf`.

## Request options

| Field | Values | Default |
| --- | --- | --- |
| `format` | `A4`, `Letter`, `Legal`, `Custom` | `A4` |
| `width_mm`, `height_mm` | 10–2000, required for `Custom` | format dimensions |
| `landscape` | boolean | `false` |
| `margins.*_mm` | 0–100 | `0` |
| `scale` | 0.1–2 | `1` |
| `print_background` | boolean | `true` |
| `prefer_css_page_size` | boolean | `true` |
| `header_template`, `footer_template` | Chrome print templates | empty |
| `page_ranges` | `1-3, 5` | all pages |
| `tagged_pdf` | boolean | `false` |
| `document_outline` | boolean | `false` |
| `wait_until` | `none`, `fonts`, `assets` | `assets` |
| `render_delay` | Go duration up to `5s` | `0s` |

Asset readiness uses page-side promises and therefore applies when `ALLOW_JAVASCRIPT=true`. With JavaScript disabled, the document is printed after the DOM is ready plus any configured render delay.

The complete contract is available in [api/openapi.yaml](api/openapi.yaml) and from the running service at `GET /openapi.yaml`.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `ADDRESS` | `:${PORT}` | HTTP listen address |
| `PORT` | `3001` | Compatibility port setting |
| `CHROME_PATH` | auto-detected | Chrome or Chromium executable |
| `CHROME_NO_SANDBOX` | `false` | Disable the Chrome sandbox |
| `ALLOW_JAVASCRIPT` | `false` | Allow scripts in submitted HTML |
| `ALLOWED_ASSET_HOSTS` | empty | Comma-separated exact remote hostnames |
| `ALLOWED_ORIGINS` | empty | Comma-separated CORS origins |
| `API_KEY` | empty | Accepted as `X-API-Key` or bearer token |
| `OPENAPI_PATH` | `api/openapi.yaml` | OpenAPI contract loaded at startup |
| `MAX_CONCURRENT_RENDERS` | `2` | Maximum active Chrome tabs |
| `MAX_REQUEST_BYTES` | `2097152` | Maximum JSON request size |
| `MAX_PDF_BYTES` | `20971520` | Maximum returned PDF size |
| `QUEUE_TIMEOUT` | `5s` | Maximum wait for a render slot |
| `RENDER_TIMEOUT` | `30s` | Maximum render duration |
| `READY_TIMEOUT` | `3s` | Chrome readiness probe timeout |
| `READ_HEADER_TIMEOUT` | `5s` | HTTP header timeout |
| `READ_TIMEOUT` | `10s` | HTTP request timeout |
| `WRITE_TIMEOUT` | `45s` | HTTP response timeout |
| `IDLE_TIMEOUT` | `60s` | HTTP keep-alive timeout |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout |

Durations use Go syntax such as `250ms`, `5s`, or `1m`.

## Operational endpoints

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Process liveness |
| `GET /readyz` | Chrome readiness |
| `GET /version` | Version, commit, and build date |
| `GET /metrics` | Prometheus text metrics |
| `GET /docs/` | Interactive Swagger UI |
| `GET /openapi.yaml` | OpenAPI 3.1 contract |

## Security

HTML is an active and potentially hostile input format. Remote network access and JavaScript are denied by default. Exact asset hosts can be enabled with `ALLOWED_ASSET_HOSTS`; deployment-level egress controls are still recommended.

For an internet-facing deployment, set an API key, configure a reverse-proxy rate limit and TLS, restrict CORS origins, protect operational endpoints, and retain the container restrictions from `docker-compose.yml`.

See [SECURITY.md](SECURITY.md) for the threat model and vulnerability reporting process.

## Development

```bash
make test
make race
make vet
make integration
```

The browser integration test is skipped unless `GO_PDF_RENDERER_INTEGRATION=1` is set.

The architecture is described in [docs/architecture.md](docs/architecture.md). Contribution guidelines are in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
