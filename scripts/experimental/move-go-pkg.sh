#! /bin/bash

set -x

usage() {
  echo "./move-go-pkg.sh SRC_PACKAGE TARGET_PACKAGE BOSH_RELEASE_DIR"
}

scripts_path=./$(dirname $0)/..
eval $($scripts_path/get_paths.sh)


src_component=$1
target_component=$2
bosh_release_dir=$3

if [ -z ${src_component} ]; then
  >&2 echo "ERROR: Specify a source package"
  usage
  exit 1
fi

if [ -z ${target_component} ]; then
  >&2 echo "ERROR: Specify a target package"
  usage
  exit 1
fi

if [ -z ${bosh_release_dir} ]; then
  >&2 echo "ERROR: Specify a bosh_release_dir"
  usage
  exit 1
fi

pushd $bosh_release_dir
  set -e
  gomvpkg -from ${src_component} -to ${target_component}
  set +e

  list=`egrep -lir ${src_component} src/ --exclude="*.md" --exclude="*.test" --exclude=".git"`
  echo $list | xargs -n 1 sed -i 's|'${src_component}'|'${target_component}'|g'
  echo $list | xargs -n 1 gofmt -w

  egrep -lir ${src_component} packages/ --exclude="*.md" --exclude="*.test" | xargs -n 1 sed -i 's|'${src_component}'|'${target_component}'|g'
  egrep -lir ${src_component} scripts/ --exclude="*.md" --exclude="*.test" | xargs -n 1 sed -i 's|'${src_component}'|'${target_component}'|g'
  egrep -lir ${src_component} jobs/ --exclude="*.md" --exclude="*.test" | xargs -n 1 sed -i 's|'${src_component}'|'${target_component}'|g'

  rm -f src/${target_component}/.git
popd

pushd ~/go
  GOPATH=$PWD
  go get ${target_component}
  cp -r src/${target_component}/.git ${bosh_release_dir}/src/${target_component}
popd

pushd $bosh_release_dir
  GOPATH=$bosh_release_dir
  for i in `git status | grep "modified content" | grep "src" | awk '{print $2}'`; do
    pushd $i
      git stash
      git co master
      git pull -r
      git stash pop
      git ci -av
    popd
  done

  pushd src/${target_component}
    git ci -av
  popd

  ./scripts/sync-submodule-config
  ./scripts/sync-package-specs

  git add packages
  ./scripts/commit-with-submodule-log
popd
