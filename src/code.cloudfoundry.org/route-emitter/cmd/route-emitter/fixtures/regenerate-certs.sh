#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"
pushd "${this_dir}"

rm -rf out
certstrap init --common-name "CA" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "client" --domain "client" --passphrase ""
certstrap sign client --CA "CA"

certstrap request-cert --common-name "metron" --domain "metron" --passphrase ""
certstrap sign metron --CA "CA"

rm -rf ./metron
mkdir -p ./metron
mv -f out/* ./metron/
rm -rf out

certstrap init --common-name "server-ca" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "server" --domain "*.bbs.service.cf.internal" --ip "127.0.0.1" --passphrase ""
certstrap sign server --CA "server-ca"

certstrap request-cert --common-name "client" --domain "client" --ip "127.0.0.1" --passphrase ""
certstrap sign client --CA "server-ca"

rm -rf ./green-certs
mkdir -p ./green-certs
mv -f out/* ./green-certs/
rm -rf out

certstrap init --common-name "ca" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "server" --passphrase "" --domain "localhost" --ip "127.0.0.1"
certstrap sign server --CA "ca"

mv -f out/* ./
rm -rf out

popd
