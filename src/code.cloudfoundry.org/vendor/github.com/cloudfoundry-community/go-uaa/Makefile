.PHONY: all build ci format ginkgo test

all: test build

format:
		go fmt ./...

ginkgo:
		go run github.com/onsi/ginkgo/v2/ginkgo -v -r --randomize-suites --randomize-all -race

test: format ginkgo

ci: ginkgo

build:
		go build ./...
