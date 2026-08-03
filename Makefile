.PHONY: build test race integration vet lint vuln fmt check docker

build:
	go build -trimpath -o dist/go-pdf-renderer ./cmd/go-pdf-renderer

test:
	go test -coverprofile=coverage.out ./...

race:
	go test -race ./...

integration:
	GO_PDF_RENDERER_INTEGRATION=1 go test -count=1 -run TestChromeRendererIntegration -v ./internal/pdf

vet:
	go vet ./...

lint:
	golangci-lint run ./...

vuln:
	go build -trimpath -o /tmp/go-pdf-renderer-vuln ./cmd/go-pdf-renderer
	govulncheck -mode=binary /tmp/go-pdf-renderer-vuln

fmt:
	gofmt -w cmd internal

check: fmt vet race

docker:
	docker build -t go-pdf-renderer:local .
