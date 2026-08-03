#!/bin/bash

set -e

this_dir="$(cd $(dirname $0) && pwd)"

pushd "$this_dir"

rm -rf out
certstrap init --common-name "ca" --passphrase ""
certstrap request-cert --common-name "multiple-ca" --passphrase ""
certstrap sign multiple-ca --CA "ca"

mv -f out/* ./instance-id/
rm -rf out

certstrap init --common-name "ca" --passphrase ""
certstrap request-cert --common-name "client" --passphrase ""
certstrap sign client --CA "ca"

mv -f out/* ./downloader/
rm -rf out

certstrap init --common-name "extra-ca" --passphrase ""

mv -f out/* ./systemcerts/
rm -rf out

popd
