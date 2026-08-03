#!/usr/bin/env bash

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

age='2 years'

pushd "${SCRIPT_DIR}"
  certstrap init   --expires "${age}" --cn CA --passphrase ''
  certstrap request-cert --cn server --ip 127.0.0.1 --passphrase ''
  certstrap sign   --expires "${age}" --CA CA server
  certstrap request-cert --cn client --ip 127.0.0.1 --passphrase ''
  certstrap sign   --expires "${age}" --CA CA client

  rm -rf cacerts certs
  mkdir cacerts certs
  mv out/CA.{crt,key} cacerts/
  mv out/client.{crt,key} out/server.{crt,key} certs/
  rm -rf out/
popd
