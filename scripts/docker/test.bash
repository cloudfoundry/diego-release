#!/bin/bash

set -eu
set -o pipefail

. "/ci/shared/helpers/git-helpers.bash"

function test() {
  local package="${1:?Provide a package}"
  local sub_package="${2:-}"

  export DIR=${package}
  . <(/ci/shared/helpers/extract-default-params-for-task.bash /ci/shared/tasks/run-bin-test/linux.yml)

  export GOFLAGS="-buildvcs=false"
  export DB="${DB}"
  /ci/shared/tasks/run-bin-test/task.bash "${sub_package}"
}

pushd /repo > /dev/null
git_configure_safe_directory
REPO_NAME=$(git_get_remote_name)
export DEFAULT_PARAMS="/ci/$REPO_NAME/default-params/run-bin-test/linux.yml"
popd > /dev/null

pushd / > /dev/null
if [[ -n "${1:-}" ]]; then
  test "src/code.cloudfoundry.org/${1}" "${2:-}"
else
  internal_repos=$(yq -r '.internal_repos|.[].name' "/ci/$REPO_NAME/index.yml")
  for component in $internal_repos; do
    test "src/code.cloudfoundry.org/${component}"
  done
fi
popd > /dev/null
