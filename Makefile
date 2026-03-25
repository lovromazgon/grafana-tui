VERSION=$(shell git describe --tags --dirty --always)

.PHONY: build
build:
	go build -ldflags "-X 'main.version=${VERSION}'" -o grafana-tui ./cmd/grafana-tui

.PHONY: test
test:
	go test -v ./...

.PHONY: test-integration
test-integration:
	go test -v -tags integration -count=1 -timeout 5m ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: clean
clean:
	rm -f grafana-tui
