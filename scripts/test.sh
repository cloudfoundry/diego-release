#!/bin/bash

specified_package="${1}"

set -e -u

declare -a serial_packages=()

declare -a ignored_packages=(
  "src/code.cloudfoundry.org/auction/simulation"
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
  ginkgo --race -randomizeAllSpecs -randomizeSuites -failFast \
      -ldflags="extldflags=-WL,--allow-multiple-definition" \
       "${@}";
  rc=$?
  popd &>/dev/null
  return "${rc}"
}

if [ "${DB}" = "mysql" ]  || [ "${DB}" = "mysql-5.6" ] || [ "${DB}" = "mysql8" ]; then
  export MYSQL_USER="root"
  export MYSQL_PASSWORD="password"
fi

declare -a packages
mapfile -t packages < <(find src -type f -name '*_test.go' -print0 | xargs -0 -L1 -I{} dirname {} | sort -u)

# filter out serial_packages from packages
for i in "${serial_packages[@]}"; do
  packages=("${packages[@]//*$i*}")
done

# filter out explicitly ignored packages
for i in "${ignored_packages[@]}"; do
  packages=("${packages[@]//*$i*}")
  serial_packages=("${serial_packages[@]//*$i*}")
done

if [[ -z "${specified_package}" ]]; then
  echo "testing packages: " "${packages[@]}"
  for dir in "${packages[@]}"; do
    test_package "${dir}" -p
  done
  echo "testing serial packages: " "${serial_packages[@]}"
  for dir in "${serial_packages[@]}"; do
    test_package "${dir}"
  done
else
  specified_package="${specified_package#./}"
  if containsElement "${specified_package}" "${serial_packages[@]}"; then
    echo "testing serial package ${specified_package}"
    test_package "${specified_package}"
  else
    echo "testing package ${specified_package}"
    test_package "${specified_package}" -p
  fi
fi
