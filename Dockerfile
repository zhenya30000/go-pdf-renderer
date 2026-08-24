FROM golang:1.27-alpine AS build

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" -o /out/go-pdf-renderer ./cmd/go-pdf-renderer

FROM debian:bookworm-slim

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked --mount=type=cache,target=/var/lib/apt/lists,sharing=locked apt-get update && apt-get install -y --no-install-recommends ca-certificates chromium-headless-shell curl fonts-dejavu-core fonts-liberation fonts-noto-core
RUN useradd --create-home --uid 10001 app

COPY --from=build /out/go-pdf-renderer /usr/local/bin/go-pdf-renderer
COPY api/openapi.yaml /usr/local/share/go-pdf-renderer/openapi.yaml

ENV ADDRESS=:3001
ENV CHROME_PATH=/usr/bin/chromium-headless-shell
ENV CHROME_NO_SANDBOX=true
ENV OPENAPI_PATH=/usr/local/share/go-pdf-renderer/openapi.yaml

USER app
EXPOSE 3001

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD curl --fail --silent http://127.0.0.1:3001/healthz >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/go-pdf-renderer"]
