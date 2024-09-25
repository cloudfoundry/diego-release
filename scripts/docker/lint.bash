#!/bin/bash

set -eu
set -o pipefail

. "/ci/shared/helpers/git-helpers.bash"
. <(/ci/shared/helpers/extract-default-params-for-task.bash /ci/shared/tasks/lint-repo/linux.yml)

pushd /repo > /dev/null
git_configure_safe_directory
REPO_NAME=$(git_get_remote_name)
export DEFAULT_PARAMS="/ci/$REPO_NAME/default-params/lint-repo/linux.yml"
popd > /dev/null

pushd / > /dev/null
/ci/shared/tasks/lint-repo/task.bash
popd > /dev/null
