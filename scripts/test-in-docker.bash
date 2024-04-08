#!/bin/bash

set -eu

THIS_FILE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
CI="${THIS_FILE_DIR}/../../wg-app-platform-runtime-ci"
. "$CI/shared/helpers/git-helpers.bash"
REPO_NAME=$(git_get_remote_name)

if [[ ${DB:-empty} == "empty" ]]; then
  DB=mysql
fi
CONTAINER_NAME="$REPO_NAME-$DB-docker-container"

DB="${DB}" "${THIS_FILE_DIR}/create-docker-container.bash" -d

docker exec $CONTAINER_NAME '/repo/scripts/docker/build-binaries.bash'
docker exec $CONTAINER_NAME '/repo/scripts/docker/tests-templates.bash'
docker exec $CONTAINER_NAME '/repo/scripts/docker/test.bash' "$@"
docker exec $CONTAINER_NAME '/repo/scripts/docker/lint.bash'
