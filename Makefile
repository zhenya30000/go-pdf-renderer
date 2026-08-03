.PHONY: build test race integration vet lint vuln fmt check docker

build:
	go build -trimpath -o dist/pdf-generator ./cmd/pdf-generator

test:
	go test -coverprofile=coverage.out ./...

race:
	go test -race ./...

integration:
	PDF_GENERATOR_INTEGRATION=1 go test -count=1 -run TestChromeRendererIntegration -v ./internal/pdf

vet:
	go vet ./...

lint:
	golangci-lint run ./...

vuln:
	go build -trimpath -o /tmp/pdf-generator-vuln ./cmd/pdf-generator
	govulncheck -mode=binary /tmp/pdf-generator-vuln

fmt:
	gofmt -w cmd internal

check: fmt vet race

docker:
	docker build -t pdf-generator:local .
