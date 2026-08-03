# go-loggregator
[![GoDoc][go-doc-badge]][go-doc]

This is a golang client library for [Loggregator][loggregator].

If you have any questions, or want to get attention for a PR or issue please reach out on the [#logging-and-metrics channel in the cloudfoundry slack](https://cloudfoundry.slack.com/archives/CUW93AF3M)

## Versions

At present, Loggregator supports two API versions: v1 (UDP) and v2 (gRPC).
This library provides clients for both versions.

Note that this library is also versioned. Its versions have *no* relation to
the Loggregator API.

## Usage

This repository should be imported as:

`import loggregator "code.cloudfoundry.org/go-loggregator/v9"`

## Examples

To build the examples, `cd` into the directory of the example and run `go build`

### V1 Ingress

Emits envelopes to metron using dropsonde.

### V2 Ingress

Emits envelopes to metron using the V2 loggregator-api.

Required Environment Variables:

* `CA_CERT_PATH`
* `CERT_PATH`
* `KEY_PATH`

### Runtime Stats

Emits information about the running Go proccess using a V2 ingress client.

Required Environment Variables:

* `CA_CERT_PATH`
* `CERT_PATH`
* `KEY_PATH`

### Envelope Stream Connector

Reads envelopes from the Loggregator API (e.g. Reverse Log Proxy).

Required Environment Variables:

* `CA_CERT_PATH`
* `CERT_PATH`
* `KEY_PATH`
* `LOGS_API_ADDR`
* `SHARD_ID`

[loggregator]:              https://github.com/cloudfoundry/loggregator-release
[go-doc-badge]:             https://godoc.org/code.cloudfoundry.org/go-loggregator?status.svg
[go-doc]:                   https://godoc.org/code.cloudfoundry.org/go-loggregator
