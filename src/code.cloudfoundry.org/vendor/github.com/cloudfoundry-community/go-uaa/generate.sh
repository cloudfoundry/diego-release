#!/bin/sh
set -ex
DIR="$(dirname "${BASH_SOURCE[0]}")"
cd "$DIR"
  go generate ./...
  find . -name '*.go' -type f -not -path './vendor/*' -exec go fmt {} \;;
cd -
