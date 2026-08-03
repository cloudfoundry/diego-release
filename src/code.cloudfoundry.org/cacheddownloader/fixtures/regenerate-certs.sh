#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"

pushd "$this_dir"

rm -rf out
certstrap init --common-name "goodCA" --passphrase ""

certstrap request-cert --common-name "server" --ip "127.0.0.1" --passphrase ""
certstrap sign server --CA "goodCA"

certstrap request-cert --common-name "localhost" --ip "127.0.0.1" --passphrase ""
certstrap sign localhost --CA "goodCA"

certstrap request-cert --common-name "goodClient" --ip "127.0.0.1" --passphrase ""
certstrap sign goodClient --CA "goodCA"

mv -f out/* .
rm -rf out

certstrap init --common-name "badCA" --passphrase ""

certstrap request-cert --common-name "badClient" --ip "127.0.0.1" --passphrase ""
certstrap sign badClient --CA "badCA"

mv -f out/* .
rm -rf out

popd
