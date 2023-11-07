#!/bin/bash

set -eu
set -o pipefail

THIS_FILE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
CI="${THIS_FILE_DIR}/../../wg-app-platform-runtime-ci"
. "$CI/shared/helpers/git-helpers.bash"
REPO_NAME=$(git_get_remote_name)
REPO_PATH="${THIS_FILE_DIR}/../"
unset THIS_FILE_DIR

if [[ ${DB:-empty} == "empty" ]]; then
  DB=mysql
fi

CONTAINER_NAME="$REPO_NAME-$DB-docker-container"
if [[ "${DB}" == "mysql" ]] || [[ "${DB}" == "mysql-8.0" ]]; then
  IMAGE="cloudfoundry/tas-runtime-mysql-8.0"
  DB="mysql"
elif [[ "${DB}" == "mysql-5.7" ]]; then
  IMAGE="cloudfoundry/tas-runtime-mysql-5.7"
  DB="mysql"
elif [[ "${DB}" == "postgres" ]]; then
  IMAGE="cloudfoundry/tas-runtime-postgres"
else
  echo "Unsupported DB flavor"
  exit 1
fi

if [[ -z "${*}" ]]; then
  ARGS="-it"
else
  ARGS="${*}"
fi

docker pull "${IMAGE}"
docker rm -f $CONTAINER_NAME
docker run -it \
  --env "DB=${DB}" \
  --env "REPO_NAME=$REPO_NAME" \
  --env "REPO_PATH=/repo" \
  --rm \
  --name "$CONTAINER_NAME" \
  -v "${REPO_PATH}:/repo" \
  -v "${CI}:/ci" \
  ${ARGS} \
  "${IMAGE}" \
  /bin/bash
  
