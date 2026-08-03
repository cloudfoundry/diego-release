#!/bin/bash

set -eu
set -o pipefail
GH_WORKSPACE=$1
BUILD_IMAGE=$2
docker run --cap-add=SYS_ADMIN --rm --privileged \
	-v "$GH_WORKSPACE:/workspace" \
	-w /workspace \
	-e MAPPING="$MAPPING" -e RUN_AS="$RUN_AS" -e FUNCTIONS="$FUNCTIONS" \
	"$BUILD_IMAGE" \
	bash ci/shared/tasks/build-binaries/task.bash
