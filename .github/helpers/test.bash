#!/bin/bash

set -eu
set -o pipefail
GH_WORKSPACE=$1
BUILD_IMAGE=$2
docker run --cap-add=SYS_ADMIN --rm --privileged \
	-v "$GH_WORKSPACE:/workspace" \
	-w /workspace \
	-e MAPPING="$MAPPING" -e DB="$DB" -e DIR="$DIR" -e RUN_AS="$RUN_AS" \
	-e VERIFICATIONS="$VERIFICATIONS" -e FUNCTIONS="$FUNCTIONS" \
	-e FLAGS="$FLAGS" \
	-e DB_USER="${DB_USER:-}" -e DB_PASSWORD="${DB_PASSWORD:-}" \
	"$BUILD_IMAGE" \
	bash ci/shared/tasks/run-bin-test/task.bash
