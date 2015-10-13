#!/bin/bash

absolute_path() {
  (cd $1 && pwd)
}

scripts_path=$(absolute_path `dirname $0`)

DIEGO_RELEASE_DIR=${DIEGO_RELEASE_DIR:-$(absolute_path $scripts_path/..)}
CF_RELEASE_DIR=${CF_RELEASE_DIR:-$(absolute_path $DIEGO_RELEASE_DIR/../cf-release)}

CF_MANIFESTS_DIR=$CF_RELEASE_DIR/bosh-lite/deployments
DIEGO_MANIFESTS_DIR=$DIEGO_RELEASE_DIR/bosh-lite/deployments

echo DIEGO_RELEASE_DIR=$DIEGO_RELEASE_DIR
echo CF_RELEASE_DIR=$CF_RELEASE_DIR
echo CF_MANIFESTS_DIR=$CF_MANIFESTS_DIR
echo DIEGO_MANIFESTS_DIR=$DIEGO_MANIFESTS_DIR
