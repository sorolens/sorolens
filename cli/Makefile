VERSION ?= dev
MODULE  := github.com/sorolens/sorolens/cli/cmd
LDFLAGS := -ldflags "-X $(MODULE).Version=$(VERSION)"

.PHONY: build install test lint release-dry-run

build:
	go build $(LDFLAGS) -o sorolens ./

install:
	go install $(LDFLAGS) ./

test:
	go test ./...

lint:
	golangci-lint run

release-dry-run:
	goreleaser release --snapshot --clean
