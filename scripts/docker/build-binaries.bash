#!/bin/bash

set -eu
set -o pipefail

. "/ci/shared/helpers/git-helpers.bash"

pushd /repo > /dev/null
git_configure_safe_directory
REPO_NAME=$(git_get_remote_name)
popd > /dev/null

. <(/ci/shared/helpers/extract-default-params-for-task.bash /ci/shared/tasks/build-binaries/linux.yml)

export DEFAULT_PARAMS="/ci/$REPO_NAME/default-params/build-binaries/linux.yml"
pushd / > /dev/null
/ci/shared/tasks/build-binaries/task.bash
popd > /dev/null
