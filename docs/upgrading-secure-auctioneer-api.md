# Upgrading to a TLS-Secured Auctioneer API

The BBS communicates to the Diego auctioneer via its API.
Diego v1.1.0 and earlier serve this API over only plain HTTP, by default on port 9016.
Diego v1.2.0 and later can also configure the auctioneer API to require mutual TLS authentication.

This document explains how to upgrade existing deployments to that TLS-secured configuration without downtime. The steps in this document also assume that the deployment manifest was generated using the [manifest-generation scripts](manifest-generation.md) in this repository, and so follows the convention that all the BBS instances update first, before any auctioneer instances are updated.

## Table of Contents

1. [Generating Credentials](#generating-credentials)
1. [Switching from plain HTTP to TLS on v1.2.0 or later](#switch-plain-http-tls-1-2-0)
  1. [First deploy](#switch-plain-http-tls-1-2-0-deploy-1)
  1. [Second deploy](#switch-plain-http-tls-1-2-0-deploy-2)


## <a name="generating-credentials"></a>Generating Credentials

Follow the instructions in the [TLS Configuration](tls-configuration.md) document to generate server and client credentials for the rep. If deploying to AWS using this repository's [AWS example](../examples/aws), follow the updated instructions there to generate these credentials.


## <a name="switch-plain-http-tls-1-2-0"></a>Switching from plain HTTP to TLS on v1.2.0 or later

It is straightforward to switch an existing Diego deployment with insecure communication to the auctioneer to one secured with mutual TLS authentication, although to avoid downtime it requires two separate deploys to secure both clients and servers. These instructions assume that the deploys below deploy Diego v1.2.0 or later, as the required configuration properties are available only in those versions.


### <a name="switch-plain-http-tls-1-2-0-deploy-1"></a>First deploy

In the property-overrides stub file supplied to the `-p` flag on the manifest-generation script, set the following properties to the contents of the credential files generated above:

1. `property_overrides.auctioneer.ca_cert`
1. `property_overrides.auctioneer.server_cert`
1. `property_overrides.auctioneer.server_key`
1. `property_overrides.auctioneer.client_cert`
1. `property_overrides.auctioneer.client_key`

With this configuration, the BBS auctioneer clients will first obtain TLS configuration, but will still be capable of communicating with insecure auctioneer servers. The auctioneer servers will then update to require mutual TLS authentication. At this point the system is secured from unauthenticated requests.

After setting these values, regenerate the deployment manifest and deploy the new version of Diego with this configuration.


### <a name="switch-plain-http-tls-1-2-0-deploy-2"></a>Second deploy

In the property-overrides stub file, now set `property_overrides.bbs.auctioneer.require_tls` to `true`.

With this configuration, the BBS auctioneer clients will require mutual TLS authentication when communicating to the auctioneer API.

After setting these values, regenerate the deployment manifest and deploy the new version of Diego with this configuration.

**NOTE**: Switching from TLS to plain HTTP is also possible, but because of the asymmetry in how the BBS and auctioneer instances update it is not as simple as reversing these steps, and cannot be done directly through the manifest-generation templates.