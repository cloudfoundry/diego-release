---
title: TLS Configuration
expires_at: never
tags: [diego-release]
---

## <a name="tls-configuration"></a>TLS Configuration

TLS with mutual authentication has to be enabled (as of version 2.0) for
communication to the BBS server. The operator must provide TLS certificates and
keys for the BBS server and its clients (other components in the Diego
deployment). This requires operators to supply `bbs.ca_cert`,
`bbs.server_cert`, and `bbs.server_key` for BBS and the BBS client cert and
key for all clients.

TLS with mutual authentication is required (as of version 2.0) for
communication to the Rep servers on the cell vms. The operator must provide TLS
certificates and keys for the rep server (`tls.ca_cert`, `tls.cert` &
`tls.key`), and its clients (other components in the Diego deployment).

**Note** If the Rep certificates have been generated prior to v1.x of Diego, those certificates will have to be regenerated with the following SANs. Failure to do so would cause evacuation to fail:
- `127.0.0.01`
- `localhost`

TLS with mutual authentication is now required for the Auctioneer (as of version 2.0)
. All the following properties are now required to be set `diego.auctioneer.ca_cert`,
`diego.auctioneer.server_cert`, `diego.auctioneer.server_key`. Also the following client certificates
and keys job properties have to be set: `diego.bbs.auctioneer.ca_cert`,
`diego.bbs.auctioneer.client_cert`, `diego.bbs.auctioneer.client_key`.
The operator may also set `diego.bbs.auctioneer.require_tls` to `true` to ensure
that all communication between the BBS and the Auctioneer server is secured using TLS
with mutual authentication.

TLS with mutual authentication can be enabled for upload and download of assets
into the containers, via the presence of the following properties:
`tls.ca_cert`, `tls.cert`, `tls.key`. See below for instructions on how to
generate those certs and keys.

### Generating TLS Certificates

#### Using BOSH cli

Bosh CLI v2 is able to automatically interpolate and generate certificates in a given deployment manifest. For more details see [Bosh CLI Interpolation](http://bosh.io/docs/cli-int.html) and [Certificate Variables](http://bosh.io/docs/variable-types.html#certificate).

[CF-Deployment](https://github.com/cloudfoundry/cf-deployment) is the canonical
open source deployment manifest for CF and is the recommended way to deploy
Diego. Below are links to the component certificates in
[CF-Deployment](https://github.com/cloudfoundry/cf-deployment):

- [Locket server certificates](https://github.com/cloudfoundry/cf-deployment/blob/20e949909c1136753f4b43e532c04c3cf02f64ac/cf-deployment.yml#L1593-L1602)
- [Locket client certificates](https://github.com/cloudfoundry/cf-deployment/blob/20e949909c1136753f4b43e532c04c3cf02f64ac/cf-deployment.yml#L1603-L1609)
- [Rep server certificates](https://github.com/cloudfoundry/cf-deployment/blob/20e949909c1136753f4b43e532c04c3cf02f64ac/cf-deployment.yml#L1439-L1451)
- [Rep client certificates](https://github.com/cloudfoundry/cf-deployment/blob/20e949909c1136753f4b43e532c04c3cf02f64ac/cf-deployment.yml#L1432-L1438)
- [Auctioneer server and client certificates](https://github.com/cloudfoundry/cf-deployment/blob/20e949909c1136753f4b43e532c04c3cf02f64ac/cf-deployment.yml#L1397-L1413)
- [BBS server and client certificates](https://github.com/cloudfoundry/cf-deployment/blob/20e949909c1136753f4b43e532c04c3cf02f64ac/cf-deployment.yml#L1414-L1431)

#### TLS Certificates for Loggregator V2 API

Since
[loggregator release version 75](https://github.com/cloudfoundry/loggregator/releases/tag/v75) metron
supports the loggregator V2 API which uses gRPC and supports TLS.

In order to enable the loggregator V2 API you need to set the following
properties:

 * `loggregator.use_v2_api`: Set this to true
 * `loggregator.v2_api_port`: Set this to the loggregator gRPC port
   (`metron_agent.grpc_port`), this property has a default value that matches
   the default value of `metron`'s
 * `loggregator.ca_cert`: Set this to the CA used to sign `metron`'s TLS
   certificates
 * `loggregator.cert`: Generate and sign a certificate using the same CA used
   above. This field is reserved for the public certificate. Instructions on
   how to generate the certs are given below.
 * `loggregator.key`: Generate and sign a certificate using the same CA
   used above. This field is reserved for the private key. Instructions on
   how to generate the certs are given below.

**NOTE:** The properties listed above need to be configured on the `rep`
template of Diego. Differently to the other properties referenced in this
document these are not global as that way of configuring BOSH is deprecated.

Assuming the loggregator ca cert and key are located at
`/path/to/loggregator-ca.crt` and `/path/to/loggregator-ca.key`, respectively.  Run
the following commands to generate the client cert/key used by the rep:

``` shell
certstrap --depot-path /path/to request-cert --cn metron-client
certstrap --depot-path /path/to sign --CA loggregator-ca metron-client
```
