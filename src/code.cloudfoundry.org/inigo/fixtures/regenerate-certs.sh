#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"

pushd "$this_dir"

# ./server/* certs
rm -rf out
certstrap init --common-name "ca" --passphrase "" -c "USA" -o "Cloud Foundry"
certstrap request-cert --common-name "server" --passphrase ""  -c "USA" -o "Cloud Foundry" --domain "gorouter.cf.service.internal"
certstrap sign server --CA "ca"
mv -f out/* ./certs/server/
rm -rf out

certstrap init --common-name "wrong-ca" --passphrase "" -c "USA" -o "Cloud Foundry"
certstrap request-cert --common-name "wrong-server" --passphrase "" --domain "127.0.0.1"
certstrap sign wrong-server --CA "wrong-ca"

mv -f out/* ./certs/server/

certstrap init --common-name "wrong-ca" --passphrase ""

mv -f out/* ./certs/
rm -rf out

certstrap init --common-name "CA" --passphrase ""
certstrap request-cert --common-name "metron" --domain "metron" --passphrase ""
certstrap sign metron --CA "CA"
certstrap request-cert --common-name "client" --domain "client" --passphrase ""
certstrap sign client --CA "CA"

mv -f out/* ./certs/metron

certstrap init --common-name "server-ca" --passphrase ""
certstrap request-cert --common-name "localhost" --domain "localhost" --passphrase ""
certstrap sign localhost --CA "server-ca"

mv -f out/localhost.crt ./certs/sql-certs/server.crt
mv -f out/localhost.csr ./certs/sql-certs/server.csr
mv -f out/localhost.key ./certs/sql-certs/server.key
mv -f out/* ./certs/sql-certs
rm -rf out

popd
