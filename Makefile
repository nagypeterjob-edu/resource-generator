UNAME := $(shell uname)

ifeq ($(UNAME), Linux)
target := linux
endif
ifeq ($(UNAME), Darwin)
target := darwin
endif

lint:
	golangci-lint run

test:
	go test -count=1 -race -cover -v $(shell go list ./... | grep -v -e /vendor/)

deploy:
	curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh
    curl -sL https://git.io/goreleaser | bash