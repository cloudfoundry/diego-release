#!/bin/bash

set -x

specified_package="${1}"
shift

set -e -u

declare -a serial_packages=()

declare -a ignored_packages=(
  "auction/simulation"
  "benchmarkbbs"
  "credhub-cli"
  "routing-api"
  "inigo"
  "cf-tcp-touer"
  "diego-upgrade-stability-tests"
  "diegocanaryapp"
  "dropsonde"
  "go-loggregator"
  "gorouter"
  "vizzini"
  "systemcerts"
)

containsElement() {
  local e match="$1"
  shift
  for e; do [[ "$e" == "$match" ]] && return 0; done
  return 1
}

test_package() {
  local package=$1
  if [ -z "${package}" ]; then
    return 0
  fi
  shift
  pushd "${package}" &>/dev/null
  ginkgo --race -randomize-all -randomize-suites -fail-fast \
      -ldflags="extldflags=-WL,--allow-multiple-definition" \
       "${@}";
  rc=$?
  popd &>/dev/null
  return "${rc}"
}

if [ "${DB}" = "mysql" ]  || [ "${DB}" = "mysql-5.6" ] || [ "${DB}" = "mysql8" ]; then
  export MYSQL_USER="root"
  export MYSQL_PASSWORD="password"
elif [ "${DB}" = "postgres" ]; then
  export POSTGRES_USER="postgres"
  export POSTGRES_PASSWORD="password"
fi

declare -a packages
pushd $DIEGO_RELEASE_DIR/src/code.cloudfoundry.org &>/dev/null
  mapfile -t packages < <(find . -type f -name '*_test.go' -print0 | xargs -0 -L1 -I{} dirname {} | sort -u)
popd &>/dev/null

# filter out serial_packages from packages
for i in "${serial_packages[@]}"; do
  packages=("${packages[@]//*$i*}")
done

# filter out explicitly ignored packages
for i in "${ignored_packages[@]}"; do
  packages=("${packages[@]//*$i*}")
  serial_packages=("${serial_packages[@]//*$i*}")
done

pushd $DIEGO_RELEASE_DIR/src/code.cloudfoundry.org
  ginkgo --race -randomize-all -randomize-suites -fail-fast -p \
    -ldflags="extldflags=-WL,--allow-multiple-definition" \
    "${packages[@]}" "${@}"
popd
