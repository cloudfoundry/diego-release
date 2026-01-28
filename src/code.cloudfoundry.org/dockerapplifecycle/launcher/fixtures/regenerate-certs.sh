#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"

pushd "$this_dir"

rm -rf out
certstrap init --common-name "credhubtest" --passphrase ""
certstrap request-cert --common-name "credhubclient" --passphrase "" --ip "127.0.0.1"
certstrap sign credhubclient --CA "credhubtest"
certstrap request-cert --common-name "credhubserver" --passphrase "" --ip "127.0.0.1"
certstrap sign credhubserver --CA "credhubtest"
certstrap init --common-name "invalid-ca" --passphrase ""
certstrap request-cert --common-name "invalid" --passphrase "" --ip "127.0.0.1"
certstrap sign invalid --CA "invalid-ca"

mv -f out/credhubtest* ./ca-certs/
mv -f out/* .
rm -rf out
popd
