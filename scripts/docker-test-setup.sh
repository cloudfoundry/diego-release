#!/bin/bash

go version # so we see the version tested in CI

set -e -u

SCRIPT_PATH="$(cd "$(dirname "${0}")" && pwd)"
. "${SCRIPT_PATH}/start-db-helper"
. "${SCRIPT_PATH}/get_paths.sh"

cd /diego-release

if ! [ $(type -P "ginkgo") ]; then
  go install -mod=mod github.com/onsi/ginkgo/ginkgo@v1
  mv /root/go/bin/ginkgo /usr/local/bin/ginkgo
fi

bootDB "${DB:-"notset"}"
set +e
exec /bin/bash "$@"
