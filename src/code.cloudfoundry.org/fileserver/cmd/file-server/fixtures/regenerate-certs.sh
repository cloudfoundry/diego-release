#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"

pushd "$this_dir"

rm -rf out
certstrap init --common-name "ca" --passphrase ""
certstrap request-cert --common-name "client" --domain "client" --passphrase "" --ip "127.0.0.1"
certstrap sign client --CA "ca"

certstrap request-cert --common-name "server" --domain "server" --passphrase "" --ip "127.0.0.1"
certstrap sign server --CA "ca"

mv -f out/* ./blue-certs/
rm -rf out

certstrap init --common-name "ca" --passphrase ""
certstrap request-cert --common-name "client" --domain "client" --passphrase "" --ip "127.0.0.1"
certstrap sign client --CA "ca"

certstrap request-cert --common-name "server" --domain "server" --passphrase "" --ip "127.0.0.1"
certstrap sign server --CA "ca"

mv -f out/* ./green-certs/
rm -rf out

certstrap init --common-name "CA" --passphrase ""
certstrap request-cert --common-name "client" --domain "client" --passphrase ""
certstrap sign client --CA "CA"

certstrap request-cert --common-name "metron" --domain "metron" --passphrase ""
certstrap sign metron --CA "CA"

mv -f out/* ./metron/
rm -rf out

popd
