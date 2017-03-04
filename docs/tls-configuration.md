## <a name="tls-configuration"></a>TLS Configuration

TLS with mutual authentication can be enabled for communication to the BBS
server, via the `diego.bbs.require_ssl` and `diego.CLIENT.bbs.require_ssl` BOSH
properties. These properties default to `true`. When enabled, the operator must
provide TLS certificates and keys for the BBS server and its clients (other
components in the Diego deployment).

TLS with mutual authentication can be enabled for communication to
the rep servers on the cell vms, via the `diego.rep.require_tls` and
`diego.CLIENT.rep.require_tls` BOSH properties. These properties default to
`false`. When enabled, the operator must provide TLS certificates and keys for
the rep server and its clients (other components in the Diego deployment).

TLS with mutual authentication can be enabled for communication to the Auctioneer
server, via the presence of any of the following properties: `diego.auctioneer.ca_cert`,
`diego.auctioneer.server_cert`, `diego.auctioneer.server_key`. If TLS enabled for
the Auctioneer, the operator must also specify the client certificates and keys
required for mutual authentication in the following properties: `diego.bbs.auctioneer.ca_cert`,
`diego.bbs.auctioneer.client_cert`, `diego.bbs.auctioneer.client_key`.
The operator may also set `diego.bbs.auctioneer.require_tls` to `true` to ensure
that all communication between the BBS and the Auctioneer server is secured using TLS
with mutual authentication.


### Generating TLS Certificates

For generating TLS certificates, we recommend
[certstrap](https://github.com/square/certstrap). An operator can follow the
following steps to successfully generate the required certificates.

> Most of these commands can be found in
> [scripts/generate-diego-certs](scripts/generate-diego-certs) as it calls
> [scripts/generate-diego-ca-certs](scripts/generate-diego-ca-certs),
> [scripts/generate-bbs-certs](scripts/generate-bbs-certs),
> [scripts/generate-rep-certs](scripts/generate-rep-certs), and
> [scripts/generate-auctioneer-certs](scripts/generate-auctioneer-certs).

1. Install certstrap from source.
   ```bash
   go get github.com/square/certstrap
   cd $GOPATH/src/github.com/square/certstrap
   ./build
   cd bin
   ```

2. Initialize a new certificate authority.
   ```bash
   $ ./certstrap init --common-name "diegoCA"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/diegoCA.key
   Created out/diegoCA.crt
   ```

   The manifest property `properties.diego.bbs.ca_cert` should be set to the
   certificate in `out/diegoCA.crt`.

3. Create and sign a certificate for the BBS server.
   ```
   $ ./certstrap request-cert --common-name "bbs.service.cf.internal" --domain "*.bbs.service.cf.internal,bbs.service.cf.internal"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/bbs.service.cf.internal.key
   Created out/bbs.service.cf.internal.csr

   $ ./certstrap sign bbs.service.cf.internal --CA diegoCA
   Created out/bbs.service.cf.internal.crt from out/bbs.service.cf.internal.csr signed by out/diegoCA.key
   ```

   The manifest property `properties.diego.bbs.server_cert` should be set to the certificate in `out/bbs.service.cf.internal.crt`.
   The manifest property `properties.diego.bbs.server_key` should be set to the certificate in `out/bbs.service.cf.internal.key`.

4. Create and sign a certificate for BBS clients.
   ```
   $ ./certstrap request-cert --common-name "clientName"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/clientName.key
   Created out/clientName.csr

   $ ./certstrap sign clientName --CA diegoCA
   Created out/clientName.crt from out/clientName.csr signed by out/diegoCA.key
   ```

   For each component `CLIENT` that has a BBS client, the manifest properties
   `properties.diego.CLIENT.bbs.client_cert` should be set to the certificate in
   `out/clientName.crt`, and the manifest properties
   `properties.diego.CLIENT.bbs.client_key` should be set to the certificate in
   `out/clientName.key`.

5. Create and sign a certificate for the Rep server.
   ```
   $ ./certstrap request-cert --common-name "cell.service.cf.internal" --domain "*.cell.service.cf.internal,cell.service.cf.internal"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/cell.service.cf.internal.key
   Created out/cell.service.cf.internal.csr

   $ ./certstrap sign cell.service.cf.internal --CA diegoCA
   Created out/cell.service.cf.internal.crt from out/cell.service.cf.internal.csr signed by out/diegoCA.key
   ```

   The manifest property `properties.diego.rep.server_cert` should be set to the certificate in `out/cell.service.cf.internal.crt`.
   The manifest property `properties.diego.rep.server_key` should be set to the certificate in `out/cell.service.cf.internal.key`.

6. Create and sign a certificate for Rep clients.
   ```
   $ ./certstrap request-cert --common-name "clientName"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/clientName.key
   Created out/clientName.csr

   $ ./certstrap sign clientName --CA diegoCA
   Created out/clientName.crt from out/clientName.csr signed by out/diegoCA.key
   ```

   For each client of the rep (i.e. `auctioneer` and `bbs`), the manifest
   properties `properties.diego.CLIENT.rep.client_cert` should be set to the
   certificate in `out/clientName.crt`, and the manifest properties
   `properties.diego.CLIENT.rep.client_key` should be set to the certificate in
   `out/clientName.key`. Where possible values for `CLIENT` are `auctioneer`
   and `bbs`.

7. Create and sign a certificate for the Auctioneer server.
   ```
   $ ./certstrap request-cert --common-name "auctioneer.service.cf.internal" --domain "auctioneer.service.cf.internal"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/auctioneer.service.cf.internal.key
   Created out/auctioneer.service.cf.internal.csr

   $ ./certstrap sign auctioneer.service.cf.internal --CA diegoCA
   Created out/auctioneer.service.cf.internal.crt from out/auctioneer.service.cf.internal.csr signed by out/diegoCA.key
   ```

   The manifest property `properties.diego.auctioneer.server_cert` should be set to the certificate in `out/auctioneer.service.cf.internal.crt`.
   The manifest property `properties.diego.auctioneer.server_key` should be set to the certificate in `out/auctioneer.service.cf.internal.key`.

8. Create and sign a certificate for Auctioneer clients.
   ```
   $ ./certstrap request-cert --common-name "clientName"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/clientName.key
   Created out/clientName.csr

   $ ./certstrap sign clientName --CA diegoCA
   Created out/clientName.crt from out/clientName.csr signed by out/diegoCA.key
   ```

   For the BBS, the manifest property `properties.diego.bbs.auctioneer.client_cert` should be set to the
   certificate in `out/clientName.crt`, and the manifest property `properties.diego.bbs.auctioneer.client_key`
   should be set to the certificate in `out/clientName.key`.

#### Experimental: TLS Certificates for Loggregator V2 API

Since [loggregaor release
version 75](https://github.com/cloudfoundry/loggregator/releases/tag/v75) metron
supports the loggregaor V2 API which uses GRPC and supports TLS.
In order to enable the loggregaor V2 API you need to set the
`loggregator.tls.use_v2_api` property to `true` and configure the following
properties:

 * `loggregator.v2_api_port`: Set this to the loggregator GRPC port
   (`metron_agent.grpc_port`), this property has a default value that matches
   the default value of `metron`'s
 * `loggregator.tls.ca_cert`: Set this to the CA used to sign `metron`'s TLS
   certificates
 * `loggregator.tls.cert`: Generate and sign a certificate using the same CA
   used above. This field is reserved for the public certificate.
 * `loggregator.tls.key`: Generate and sign a certificate using the same CA
   used above. This field is reserved for the private key.

**NOTE:** The properties listed above need to be configured on the `rep`
template of Diego. Differently to the other properties referenced in this
document these are not global as that way of configuring BOSH is deprecated.

### Custom TLS Certificate Generation

If you already have a CA, or wish to use your own names for clients and
servers, please note that the common-names "diegoCA" and "clientName" are
placeholders and can be renamed.
The server certificate must have the common names. For example
`cell.service.cf.internal` and must specify `cell.service.cf.internal` and
`*.cell.service.cf.internal` as Subject Alternative Names (SANs).
