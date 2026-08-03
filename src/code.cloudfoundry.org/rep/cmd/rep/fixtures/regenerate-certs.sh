#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"

pushd "$this_dir"

certstrap init --common-name "server-ca" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "client" --domain "client" --passphrase "" --ip "127.0.0.1"
certstrap sign client --CA "server-ca"

certstrap request-cert --common-name "server" --passphrase "" --ip "127.0.0.1" --domain "*.bbs.service.cf.internal"
certstrap sign server --CA "server-ca"

mv -f out/* ./blue-certs/

certstrap init --common-name "root-ca" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "intermed-ca" --domain "intermed-ca" --passphrase "" --ip "127.0.0.1"
certstrap sign intermed-ca --CA "root-ca" --intermediate
certstrap request-cert --common-name "server" --domain "server" --passphrase "" --ip "127.0.0.1"
certstrap sign server --CA "intermed-ca"
cat out/server.crt out/intermed-ca.crt > ./chain-certs/chain.crt
cat out/server.crt > ./chain-certs/bad-chain.crt && tail -n5 out/intermed-ca.crt >> ./chain-certs/bad-chain.crt

mv -f out/* ./chain-certs/

certstrap init --common-name "CA" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "metron" --domain "metron" --passphrase ""
certstrap sign metron --CA "CA"
certstrap request-cert --common-name "client" --domain "metron" --passphrase ""
certstrap sign client --CA "CA"

mv -f out/* ./metron/

certstrap init --common-name "server-ca" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "server" --passphrase "" --domain "server"
certstrap sign server --CA "server-ca"
certstrap request-cert --common-name "client" --passphrase "" --domain "client"
certstrap sign client --CA "server-ca"

mv -f out/* ./rouge-certs/

certstrap init --common-name "bbsCA" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "server" --passphrase "" --ip "127.0.0.1" --domain "*.bbs.service.cf.internal"
certstrap sign server --CA "bbsCA"
certstrap request-cert --common-name "client" --passphrase "" --ip "127.0.0.1" --domain "client"
certstrap sign client --CA "bbsCA"
mv ./out/bbsCA.crt ./out/server-ca.crt

mv -f out/* ./green-certs/

certstrap init --common-name "server-ca" --passphrase "" --exclude-path-length
certstrap request-cert --common-name "server" --passphrase "" --domain "server"
certstrap sign server --CA "server-ca"
certstrap request-cert --common-name "client" --passphrase "" --domain "localhost"
certstrap sign client --CA "server-ca"

mv -f out/* ./dnssan-certs/
rm -rf out

popd
