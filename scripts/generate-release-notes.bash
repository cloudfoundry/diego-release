#!/bin/bash

set -eu
set -o pipefail

THIS_FILE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
CI="${THIS_FILE_DIR}/../../wg-app-platform-runtime-ci"
. "$CI/shared/helpers/release-note-helpers.bash"
. "$CI/shared/helpers/git-helpers.bash"
unset THIS_FILE_DIR

# ex. version_range="v0.343.0...v0.344.0"
version_range="${1:?Please provide the start and end versions you want to generate release notes for './generate-release-notes.bash start_ref...end_ref' }"
# ex. local_start_ref="v0.343.0"
local_start_ref=$(get_start_ref_from_range "${version_range}")
# ex. local_end_ref="v0.344.0"
local_end_ref=$(get_end_ref_from_range "${version_range}")

GO_MOD_LOCATION="src/code.cloudfoundry.org/go.mod";
BLOBS_LOCATION="config/blobs.yml";

get_non_bot_commits "${local_start_ref}" "${local_end_ref}"
echo ""

START_REF_CNBAPPLIFECYCLE=$(git rev-parse "${local_start_ref}:src/cnbapplifecycle")
END_REF_CNBAPPLIFECYCLE=$(git rev-parse "${local_end_ref}:src/cnbapplifecycle")
pushd src/cnbapplifecycle > /dev/null
  get_non_bot_commits "${START_REF_CNBAPPLIFECYCLE}" "${END_REF_CNBAPPLIFECYCLE}" "cnbapplifecycle"
popd > /dev/null
echo ""

for repo in auction auctioneer bbs buildpackapplifecycle cacheddownloader cfdot diego-ssh dockerapplifecycle ecrhelper executor fileserver healthcheck inigo localdriver locket operationq rep route-emitter vizzini volman workpool; do
  START_REF_REPO=$(git rev-parse "${local_start_ref}:src/code.cloudfoundry.org/${repo}")
  END_REF_REPO=$(git rev-parse "${local_end_ref}:src/code.cloudfoundry.org/${repo}")
  pushd src/code.cloudfoundry.org/${repo} > /dev/null
    get_non_bot_commits "${START_REF_REPO}" "${END_REF_REPO}" "${repo}"
  popd > /dev/null
  echo ""
done

display_blob_change_info "${local_start_ref}" "${local_end_ref}" "${BLOBS_LOCATION}"
echo ""

display_go_mod_diff "${local_start_ref}" "${local_end_ref}" "${GO_MOD_LOCATION}"
echo ""

for repo in cacheddownloader ; do 
  START_REF_REPO=$(git rev-parse "${local_start_ref}:src/code.cloudfoundry.org/${repo}")
  END_REF_REPO=$(git rev-parse "${local_end_ref}:src/code.cloudfoundry.org/${repo}")
  pushd src/code.cloudfoundry.org/${repo} > /dev/null
  display_go_mod_diff "${START_REF_REPO}" "${END_REF_REPO}" "go.mod" "${repo}"
  popd > /dev/null
  echo ""
done

START_REF_CNBAPPLIFECYCLE=$(git rev-parse "${local_start_ref}:src/cnbapplifecycle")
END_REF_CNBAPPLIFECYCLE=$(git rev-parse "${local_end_ref}:src/cnbapplifecycle")
pushd src/cnbapplifecycle > /dev/null
  display_go_mod_diff "${START_REF_CNBAPPLIFECYCLE}" "${END_REF_CNBAPPLIFECYCLE}" "go.mod" "cnbapplifecycle"
popd > /dev/null
echo ""
