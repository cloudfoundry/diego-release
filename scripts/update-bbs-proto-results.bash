#! /bin/bash

set -eu
set -o pipefail

THIS_FILE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

function run() {
  local task_tmp_dir="${1:?provide temp dir for task}"
  shift 1 
  local new_branch="${1:?Please provide the branch that contains the 'new' protobufs}"
  local old_branch="${2:-preserve-gogo-protobuf}"
  # shellcheck disable=2155
  local utc_timestamp="$(date --utc +%Y%m%d_%H%M%S)"

  pushd "${THIS_FILE_DIR}/../src/code.cloudfoundry.org/bbs/models/performance" > /dev/null
    local old_results_path="${task_tmp_dir}/${old_branch}_${utc_timestamp}.txt"
    git checkout "${old_branch}"
    git pull
    echo "  Running performance tests..."
    ginkgo --no-color . > "${old_results_path}"
    echo "  Complete"

    local new_results_path="${task_tmp_dir}/${new_branch}_${utc_timestamp}.txt"
    git checkout "${new_branch}"
    git pull
    echo "  Running performance tests..."
    ginkgo --no-color . > "${new_results_path}"
    echo "  Complete"

    cp "${old_results_path}" "./results/gogo-protobuf.txt"
    cp "${new_results_path}" "./results/google-protobuf.txt"
  popd > /dev/null
}

function cleanup() {
  rm -rf "${task_tmp_dir}"
}

task_tmp_dir="$(mktemp -d -t 'XXXX-bbs-proto-perf-tmp-dir')"
trap cleanup EXIT
run $task_tmp_dir "$@"
