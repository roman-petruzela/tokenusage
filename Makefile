VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test vet run clean
build:
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o tokenusage ./cmd/tokenusage
test:
	go test ./...
vet:
	go vet ./...
run:
	go run ./cmd/tokenusage
clean:
	rm -f tokenusage
