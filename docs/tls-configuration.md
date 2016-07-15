##<a name="tls-configuration"></a>TLS Configuration

Diego Release can be configured to require TLS for communication with etcd.
To enable or disable TLS communication with etcd, the `etcd.require_ssl`
and `diego.bbs.etcd.require_ssl` properties should be set to `true` or
`false`.  By default, Diego has `require_ssl` set to `true`.  When
`require_ssl` is `true`, the operator must generate TLS certificates and keys
for the etcd server and its clients.

TLS and mutual authentication can also be enabled between etcd peers. To
enable or disable this, the `etcd.peer_require_ssl` property should be
set to `true` or `false`. By default, Diego has `peer_require_ssl` set to
`true`.  When `peer_require_ssl` is set to `true`, the operator must provide
TLS certificates and keys for the cluster members. The CA, server certificate,
and server key across may be shared between the client and peer configurations
if desired.

Similarly, TLS with mutual authentication can be enabled for communication to
the BBS server, via the `diego.bbs.require_ssl` and
`diego.CLIENT.bbs.require_ssl` BOSH properties. These properties default to
`true`. When enabled, the operator must provide TLS certificates and keys for
the BBS server and its clients (other components in the Diego deployment).


### Generating TLS Certificates

For generating TLS certificates, we recommend
[certstrap](https://github.com/square/certstrap).  An operator can follow the
following steps to successfully generate the required certificates.

> Most of these commands can be found in
> [scripts/generate-diego-ca-certs](scripts/generate-diego-ca-certs),
> [scripts/generate-etcd-certs](scripts/generate-etcd-certs), and
> [scripts/generate-bbs-certs](scripts/generate-bbs-certs)


1. Get certstrap
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

   The manifest properties `properties.diego.etcd.ca_cert` and
   `properties.diego.bbs.ca_cert` should be set to the certificate in
   `out/diegoCA.crt`.

3. Create and sign a certificate for the etcd server.
   ```bash
   $ ./certstrap request-cert --common-name "etcd.service.cf.internal" --domain "*.etcd.service.cf.internal,etcd.service.cf.internal"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/etcd.service.cf.internal.key
   Created out/etcd.service.cf.internal.csr

   $ ./certstrap sign etcd.service.cf.internal --CA diegoCA
   Created out/etcd.service.cf.internal.crt from out/etcd.service.cf.internal.csr signed by out/diegoCA.key
   ```

   The manifest property `properties.etcd.server_cert` should be set to the certificate in `out/etcd.service.cf.internal.crt`.
   The manifest property `properties.etcd.server_key` should be set to the certificate in `out/etcd.service.cf.internal.key`.

4. Create and sign a certificate for etcd clients.
   ```
   $ ./certstrap request-cert --common-name "clientName"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/clientName.key
   Created out/clientName.csr

   $ ./certstrap sign clientName --CA diegoCA
   Created out/clientName.crt from out/clientName.csr signed by out/diegoCA.key
   ```

   The manifest property `properties.etcd.client_cert` should be set to the certificate in `out/clientName.crt`.
   The manifest property `properties.etcd.client_key` should be set to the certificate in `out/clientName.key`.

5. Create and sign a certificate for the BBS server.
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

6. Create and sign a certificate for BBS clients.
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

7. (Optional) Initialize a new peer certificate authority.
   ```
   $ ./certstrap --depot-path peer init --common-name "peerCA"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created peer/peerCA.key
   Created peer/peerCA.crt
   ```

   The manifest property `properties.etcd.peer_ca_cert` should be set to the certificate in `peer/peerCA.crt`.

8. (Optional) Create and sign a certificate for the etcd peers.
   ```
   $ ./certstrap --depot-path peer request-cert --common-name "etcd.service.cf.internal" --domain "*.etcd.service.cf.internal,etcd.service.cf.internal"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created peer/etcd.service.cf.internal.key
   Created peer/etcd.service.cf.internal.csr

   $ ./certstrap --depot-path peer sign etcd.service.cf.internal --CA diegoCA
   Created peer/etcd.service.cf.internal.crt from peer/etcd.service.cf.internal.csr signed by peer/peerCA.key
   ```

   The manifest property `properties.etcd.peer_cert` should be set to the certificate in `peer/etcd.service.cf.internal.crt`.
   The manifest property `properties.etcd.peer_key` should be set to the certificate in `peer/etcd.service.cf.internal.key`.


### Custom TLS Certificate Generation

If you already have a CA, or wish to use your own names for clients and
servers, please note that the common-names "diegoCA" and "clientName" are
placeholders and can be renamed provided that all clients client certificate.
The server certificate must have the common name `etcd.service.cf.internal` and
must specify `etcd.service.cf.internal` and `*.etcd.service.cf.internal` as
Subject Alternative Names (SANs).
