#!/bin/bash

absolute_path() {
  (cd $1 && pwd)
}

scripts_path=$(absolute_path `dirname $0`)
DIEGO_RELEASE_DIR=${DIEGO_RELEASE_DIR:-$(absolute_path $scripts_path/..)}
echo DIEGO_RELEASE_DIR=$DIEGO_RELEASE_DIR
