#!/bin/bash

set -eu
set -o pipefail

. <(/ci/shared/helpers/extract-default-params-for-task.bash /ci/shared/tasks/lint-repo/linux.yml)

export DEFAULT_PARAMS="/ci/$REPO_NAME/default-params/lint-repo/linux.yml"
pushd / > /dev/null
/ci/shared/tasks/lint-repo/task.bash
popd > /dev/null
