# Upgrading to a secure deployment of diego

This document explains a 2 deploy process that serves as an upgrade path to a
deployment that uses secure communication between the `Auctioner`, `BBS` and
the `Rep`.

## Generating certs

Follow the instructions in [TLS Configuration](tls-configuration.md) or if
deploying to AWS follow [AWS Readme](../examples/aws/README.md) in order to
generate server and client certificates for the rep.

## First deploy

In the `property_overrides.yml` set each of the following to the content of the
certs obtained in the previous step:

1. `property_overrides.rep.ca_cert`
2. `property_overrides.rep.server_cert`
3. `property_overrides.rep.server_key`
4. `property_overrides.rep.client_cert`
5. `property_overrides.rep.client_key`

set the following properties to `false`

1. `property_overrides.bbs.rep.require_tls`
1. `property_overrides.auctioneer.rep.require_tls`
1. `property_overrides.rep.require_tls`

Once the properties are in place, you should generate a new deployment manifest
and deploy diego.

This step ensures that all the rep clients have the necessary
certs in place. the clients (i.e. `Auctioneer` and `BBS`) will prefer tls
communication but will operate in a backward compatible mode in which they can
still talk to the rep over http.

## Second deploy

in the `property_overrides.yml` set the following property to `true`

1. `property_overrides.rep.require_tls`

Once the properties are in place, you should generate a new deployment manifest
and deploy diego.

This step will force the rep to only accept connections using
TLS. The previous deploy prepared the `Auctioneer` and `BBS` to connect to the
`Rep` over either http or https. This way they shouldn't have trouble
communicating with the different `Reps` during a rolling deploy.

### Optional Third deploy

At this point all `Auctioneer` and `BBS` communication with `Rep` will be
taking place over https. In order to ensure that `Auctioneer` and `BBS`
**only** communicate with the `Reps` over https and refuse to use http, you
should set the following properties to `true` in the `property_overrides.yml`:

1. `property_overrides.bbs.rep.require_tls`
1. `property_overrides.auctioneer.rep.require_tls`

Again, regenerate the deployment manifest and deploy diego once the properties
are in place.

**Note** once the deploy is finished, both `Auctioneer` and `BBS` will refuce
to communicate to any `Rep` over `http`.
