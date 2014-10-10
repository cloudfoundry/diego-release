#!/bin/bash

set -e -x -u

attempts=0
until bosh -n target ${BOSH_TARGET}; do
  [ $(( attempts++ )) -ge 3 ] && exit 1
  sleep 5
done

if ! bosh -n login ${BOSH_USER} ${BOSH_PASSWORD}; then
  echo "Creating director user"
  bosh -n -u admin -p admin create user ${BOSH_USER} ${BOSH_PASSWORD}
fi
