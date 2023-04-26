#!/bin/bash

go version # so we see the version tested in CI

set -e -u

SCRIPT_PATH="$(cd "$(dirname "${0}")" && pwd)"
. "${SCRIPT_PATH}/start-db-helper"
. "${SCRIPT_PATH}/get_paths.sh"

cd /diego-release

if ! [ $(type -P "ginkgo") ]; then
  go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo
  mv /root/go/bin/ginkgo /usr/local/bin/ginkgo
fi

if ! [ $(type -P "nats-server") ]; then
  BIN_DIR="${DIEGO_RELEASE_DIR}/bin"
  mkdir -p "${BIN_DIR}"
  export PATH="${PATH}:${BIN_DIR}"
  export GOFLAGS="-buildvcs=false"
  pushd "${DIEGO_RELEASE_DIR}/src/code.cloudfoundry.org"
    go build -o "$BIN_DIR/nats-server" github.com/nats-io/nats-server/v2
  popd
fi

bootDB "${DB:-"notset"}"
set +e
exec /bin/bash "$@"
