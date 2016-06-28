#!/bin/bash

set -e -x

scripts_path=./$(dirname $0)/..
eval $($scripts_path/get_paths.sh)

pushd $DIEGO_RELEASE_DIR
  for i in `git status | grep "modified content" | grep "src" | awk '{print $2}'`; do
    pushd $i
      git stash
      git co master
      git pull -r
      git stash pop
      git ci -av
    popd
  done
popd
