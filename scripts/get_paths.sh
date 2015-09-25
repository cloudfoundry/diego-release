#!/bin/bash

absolute_path() {
  (cd $1 && pwd)
}

scripts_path=$(absolute_path `dirname $0`)

DIEGO_RELEASE_DIR=${DIEGO_RELEASE_DIR:-$(absolute_path $scripts_path/..)}
CF_RELEASE_DIR=${CF_RELEASE_DIR:-$(absolute_path $DIEGO_RELEASE_DIR/../cf-release)}
MANIFESTS_DIR=${MANIFESTS_DIR:-$DIEGO_RELEASE_DIR/bosh-lite-manifests}

echo DIEGO_RELEASE_DIR=$DIEGO_RELEASE_DIR
echo CF_RELEASE_DIR=$CF_RELEASE_DIR
echo MANIFESTS_DIR=$MANIFESTS_DIR
