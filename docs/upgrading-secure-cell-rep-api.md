# Upgrading to a TLS-Secured Cell Rep API

The BBS and auctioneer both communicate to the Diego cells via the cell rep API.
Diego v0.1487.0 and earlier serve this API over only plain HTTP, by default on port 1800.
Diego v0.1488.0 and later also serve this API on a server that can be secured by mutual TLS, defaulting to port 1801.

This document explains how to upgrade existing deployments to that TLS-secured configuration without downtime. The steps in this document also assume that the deployment manifest was generated using the [manifest-generation scripts](manifest-generation.md) in this repository, and so follows the convention that all the BBS instances update first, but auctioneers and cells may update in an uncoordinated way.

## Table of Contents

1. [Generating Credentials](#generating-credentials)
1. [Upgrading to TLS from v0.1487.0](#upgrade-tls)
  1. [First deploy](#upgrade-tls-0-1487-deploy-1)
  1. [Second deploy](#upgrade-tls-0-1487-deploy-2)
1. [Upgrading to plain HTTP from v0.1487.0](#upgrade-plain-http-0-1487)
  1. [First deploy](#upgrade-plain-http-0-1487-deploy-1)
1. [Switching from plain HTTP to TLS on v0.1488.0 or later](#switch-plain-http-tls-0-1488)
  1. [First deploy](#switch-plain-http-tls-0-1488-deploy-1)
  1. [Second deploy](#switch-plain-http-tls-0-1488-deploy-2)
  1. [Third deploy](#switch-plain-http-tls-0-1488-deploy-3)


## <a name="generating-credentials"></a>Generating Credentials

Follow the instructions in the [TLS Configuration](tls-configuration.md) document to generate server and client credentials for the rep. If deploying to AWS using this repository's [AWS example](../examples/aws), follow the updated instructions there to generate these credentials.


## <a name="upgrade-tls"></a>Upgrading to TLS from v0.1487.0 

This section assumes an existing Diego deployment on v0.1487.0 or earlier that will be upgraded to Diego v0.1488.0 or later on subsequent deploys.

### <a name="upgrade-tls-0-1487-deploy-1"></a>First deploy

In the property-overrides stub file supplied to the `-p` flag on the manifest-generation script, set the following properties to the contents of the credential files generated above:

1. `property_overrides.rep.ca_cert`
1. `property_overrides.rep.server_cert`
1. `property_overrides.rep.server_key`
1. `property_overrides.rep.client_cert`
1. `property_overrides.rep.client_key`

Set the following properties to `true`:

1. `property_overrides.rep.require_tls`
1. `property_overrides.rep.enable_legacy_api_endpoints`

Set the following properties to `false`:

1. `property_overrides.bbs.rep.require_tls`
1. `property_overrides.auctioneer.rep.require_tls`

With this configuration, the updated cell reps will also serve their APIs on port 1801 secured by TLS. As the BBS and auctioneer instances update, they will prefer connecting to updated cells on this port and will have the credentials to do so, but will still fall back to the insecure server on port 1800 for cells that have not updated.

After setting these values, regenerate the deployment manifest and deploy the new version of Diego with this configuration.

### <a name="upgrade-tls-0-1487-deploy-2"></a>Second deploy

In the property-overrides stub file, now set the following properties to `true`:

1. `property_overrides.bbs.rep.require_tls`
1. `property_overrides.auctioneer.rep.require_tls`

Set the following properties to `false`:

1. `property_overrides.rep.enable_legacy_api_endpoints`

With this configuration, the BBS and auctioneer instances will connect to the cell rep APIs only via TLS.

After setting these values, regenerate the deployment manifest and deploy the new version of Diego with this configuration.


## <a name="upgrade-plain-http-0-1487"></a>Upgrading to plain HTTP from v0.1487.0

Although not recommended for a production deployment of Diego, it is also possible for the cell reps and their clients to continue to communicate over plain HTTP. Deploy as follows when upgrading from Diego v0.1487.0 or earlier:

###<a name="upgrade-plain-http-0-1487-deploy-1"></a>First deploy

In the property-overrides stub file supplied to the `-p` flag on the manifest-generation script, do NOT set values for the following properties:

1. `property_overrides.rep.ca_cert`
1. `property_overrides.rep.server_cert`
1. `property_overrides.rep.server_key`
1. `property_overrides.rep.client_cert`
1. `property_overrides.rep.client_key`

Set the following properties to `false`:

1. `property_overrides.bbs.rep.require_tls`
1. `property_overrides.auctioneer.rep.require_tls`
1. `property_overrides.rep.require_tls`

With this configuration, the updated cell reps will also serve their APIs on port 1801 over plain HTTP. As the BBS and auctioneer instances update, they will prefer connecting to updated cells on this port, but will still fall back to the insecure server on port 1800 for cells that have not updated.

After setting these values, regenerate the deployment manifest and deploy the new version of Diego with this configuration.

## <a name="switch-plain-http-tls-0-1488"></a>Switching from plain HTTP to TLS on v0.1488.0 or later

It is also possible to switch a Diego deployment on v0.1488.0 or later between plain HTTP communication and mutual TLS, although to avoid downtime it requires three separate deploy steps. We detail the steps for switching from plain HTTP to mutual TLS below; reverse them to switch back.

### <a name="switch-plain-http-tls-0-1488-deploy-1"></a>First deploy

In the property-overrides stub file supplied to the `-p` flag on the manifest-generation script, set the following properties to the contents of the credential files generated above:

1. `property_overrides.rep.ca_cert`
1. `property_overrides.rep.server_cert`
1. `property_overrides.rep.server_key`
1. `property_overrides.rep.client_cert`
1. `property_overrides.rep.client_key`

Set the following properties to `true`:

1. `property_overrides.rep.enable_legacy_api_endpoints`

Set the following properties to `false`:

1. `property_overrides.bbs.rep.require_tls`
1. `property_overrides.auctioneer.rep.require_tls`
1. `property_overrides.rep.require_tls`

With this configuration, the updated cell reps will also serve their APIs on port 1801 over plain HTTP. As the BBS and auctioneer instances update, they will still connect to the cell rep API over plain HTTP, but will now also be capable of connecting to TLS-secured ones in following deploys. 

After setting these values, regenerate the deployment manifest and deploy the new version of Diego with this configuration.


### <a name="switch-plain-http-tls-0-1488-deploy-2"></a>Second deploy

In the property-overrides stub file, now set `property_overrides.rep.require_tls` to `true`.

With this configuration, the cell reps will update to serve their APIs secured by mutual TLS. From the previous deploy, the BBS and auctioneer instances have the required credentials to connect.

After setting these values, regenerate the deployment manifest and deploy the new version of Diego with this configuration.


### <a name="switch-plain-http-tls-0-1488-deploy-3"></a>Third deploy

In the property-overrides stub file, now set the following properties to `false`:

1. `property_overrides.bbs.rep.require_tls`
1. `property_overrides.auctioneer.rep.require_tls`
1. `property_overrides.rep.enable_legacy_api_endpoints`

With this configuration, the BBS and auctioneers will connect to the cell reps only via mutual TLS. From the previous deploy, all the cell reps already serve their APIs only over TLS.

After setting these values, regenerate the deployment manifest and deploy the new version of Diego with this configuration.
