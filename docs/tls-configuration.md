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
`diego.auctioneer.server_cert`, `diego.auctioneer.server_key`. If TLS is enabled for
the Auctioneer, the operator must also specify the client certificates and keys
required for mutual authentication in the following properties: `diego.bbs.auctioneer.ca_cert`,
`diego.bbs.auctioneer.client_cert`, `diego.bbs.auctioneer.client_key`.
The operator may also set `diego.bbs.auctioneer.require_tls` to `true` to ensure
that all communication between the BBS and the Auctioneer server is secured using TLS
with mutual authentication.

TLS with mutual authentication can be enabled for upload and download of assets
into the containers, via the presence of the following properties:
`tls.ca_cert`, `tls.cert`, `tls.key`. See below for instructions on how to
generate those certs and keys.

### Generating TLS Certificates

For generating TLS certificates, we recommend
[certstrap](https://github.com/square/certstrap). An operator can follow the
following steps to successfully generate the required certificates.

> Most of these commands can be found in
> [scripts/generate-diego-certs](/scripts/generate-diego-certs) as it calls
> [scripts/generate-diego-ca-certs](/scripts/generate-diego-ca-certs),
> [scripts/generate-bbs-certs](/scripts/generate-bbs-certs),
> [scripts/generate-rep-certs](/scripts/generate-rep-certs), and
> [scripts/generate-auctioneer-certs](/scripts/generate-auctioneer-certs).

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

4. Create and sign a certificate for the Locket server.
   ```
   $ ./certstrap request-cert --common-name "locket.service.cf.internal" --domain "locket.service.cf.internal"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/locket.service.cf.internal.key
   Created out/locket.service.cf.internal.csr

   $ ./certstrap sign locket.service.cf.internal --CA diegoCA
   Created out/locket.service.cf.internal.crt from out/locket.service.cf.internal.csr signed by out/diegoCA.key
   ```

   The manifest property `tls.ca_cert` for the `locket` job should be set to
   the certificate in `out/diegoCA.crt`.
   The manifest property `tls.cert` for the `locket` job should be set to
   the certificate in `out/locket.service.cf.internal.crt`.
   The manifest property `tls.key` for the `locket` job should be set to
   the certificate in `out/locket.service.cf.internal.key`.

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

6. Create and sign a certificate for Rep component.

   The following set of properties are currently used by the
   uploader/downloader to authenticate with the blobstore and CC. If you signed
   the Rep server certificate in the previous step using the CF/Diego mutual
   TLS certificate authority, then you will be able to use the same cert/key
   for the following properties. Otherwise, you will have to generate a new
   certificate by running:

   ```
   $ ./certstrap request-cert --common-name "clientName"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/clientName.key
   Created out/clientName.csr

   $ ./certstrap sign clientName --CA cf-diego-ca # cf-diego-ca is the mutual CF/Diego CA
   Created out/clientName.crt from out/clientName.csr signed by out/cf-diego-ca.key
   ```

   **Note** the following properties must be set in the cell job spec:
   - `properties.tls.ca_cert`: The CF/Diego mutual TLS certificate authority
   - `properties.tls.cert`
   - `properties.tls.key`

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

### Custom TLS Certificate Generation

If you already have a CA, or wish to use your own names for clients and
servers, please note that the common-names "diegoCA" and "clientName" are
placeholders and can be renamed.
The server certificate must have the common names. For example
`cell.service.cf.internal` and must specify `cell.service.cf.internal` and
`*.cell.service.cf.internal` as Subject Alternative Names (SANs).
