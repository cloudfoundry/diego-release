#!/bin/bash

set -eu
set -o pipefail

pushd / > /dev/null
/ci/shared/tasks/run-tests-templates/task.bash
popd > /dev/null
