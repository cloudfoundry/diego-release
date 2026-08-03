#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"

pushd "$this_dir"

rm -rf out
certstrap init --common-name "locketCA" --passphrase "" --expires "10 years"

certstrap request-cert --common-name "locketServer" --ip "127.0.0.1" --domain "localhost" --passphrase ""
certstrap sign locketServer --CA "locketCA" --expires "10 years"

certstrap request-cert --common-name "locketClient" --ip "127.0.0.1" --passphrase ""
certstrap sign locketClient --CA "locketCA" --expires "10 years"

mv -f out/locketServer.crt ./locketServer.crt
mv -f out/locketServer.key ./locketServer.key
mv -f out/locketClient.crt ./locketClient.crt
mv -f out/locketClient.key ./locketClient.key
mv -f out/locketCA.crt ./locketCA.crt

rm -rf out



popd
