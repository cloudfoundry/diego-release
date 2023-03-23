#!/bin/bash
set -u

ROOT_DIR_PATH="$(cd $(dirname $0)/.. && pwd)"
cd "${ROOT_DIR_PATH}"

echo "Running template tests"

docker run \
   --rm \
   -it \
   --privileged \
   -v "${PWD}:/diego-release" \
   --cap-add ALL \
   -w /diego-release \
   "ruby:3.0" \
   /diego-release/scripts/template-tests "$@"

echo "Done executing template tests"