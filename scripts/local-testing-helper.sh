#!/bin/bash

set -eu

THIS_FILE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
CI="${THIS_FILE_DIR}/../../wg-app-platform-runtime-ci"
. "$CI/shared/helpers/git-helpers.bash"
REPO_NAME=$(git_get_remote_name)

function setup_docker {
    if [[ ${DB:-empty} == "empty" ]]; then
      DB=mysql
    fi

    DB="${DB}" "${THIS_FILE_DIR}/create-docker-container.bash" -d

    docker exec "${REPO_NAME}-${DB}-docker-container" '/repo/scripts/docker/build-binaries.bash'
}

function test_in_docker_locally {
  setup_docker
  docker exec "${REPO_NAME}-${DB}-docker-container" '/repo/scripts/docker/tests-templates.bash'
  docker exec "${REPO_NAME}-${DB}-docker-container" '/repo/scripts/docker/test.bash' "$@"
  docker exec "${REPO_NAME}-${DB}-docker-container" '/repo/scripts/docker/lint.bash'
}

function start_docker_for_testing {
  setup_docker
  docker exec -ti "${REPO_NAME}-${DB}-docker-container" 'bash' '--rcfile' '/repo/scripts/docker/init.bash'
}
