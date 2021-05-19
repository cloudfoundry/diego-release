#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
DIEGO_RELEASE_DIR=${DIEGO_RELEASE_DIR:-$(realpath ${SCRIPT_DIR}/..)}
export DIEGO_RELEASE_DIR=$DIEGO_RELEASE_DIR
