#!/bin/bash
set -u

SCRIPT_PATH="$(cd "$(dirname "${0}")" && pwd)"

. "${SCRIPT_PATH}/run-template-tests-in-docker.sh"
. "${SCRIPT_PATH}/docker-test"
