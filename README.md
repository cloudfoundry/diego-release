# Cloud Foundry Diego [BOSH release] [![slack.cloudfoundry.org](https://slack.cloudfoundry.org/badge.svg)](https://slack.cloudfoundry.org)

----
This repo is a [BOSH](https://github.com/cloudfoundry/bosh) release for
deploying Diego and associated tasks for testing a Diego deployment.  Diego
builds out the new runtime architecture for Cloud Foundry, replacing the DEAs
and Health Manager.

This release relies on a separate deployment to provide
[Consul](https://github.com/hashicorp/consul),
[NATS](https://github.com/apcera/gnatsd) and
[Loggregator](https://github.com/cloudfoundry/loggregator). In practice these
come from [cf-release](https://github.com/cloudfoundry/cf-release).

### Additional Diego Resources

  - The [Contribution Guidelines](CONTRIBUTING.md) describes the developer workflow for making changes to Diego.
  - The [Diego Design Notes](https://github.com/cloudfoundry-incubator/diego-design-notes) present an overview of Diego, and links to the various Diego components.
  - The [Migration Guide](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/migrating-to-diego.md) describes how developers and operators can manage a transition from the DEAs to Diego.
  - The [Docker Support Notes](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/docker-support.md) describe how Diego runs Docker-image-based apps in Cloud Foundry.
  - The [SSH Access Notes](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/ssh-access-and-policy.md) describe how Diego's SSH proxy and daemon can be used to connect to app instances running on Diego.
  - The [Diego-CF Compatibility Log](https://github.com/cloudfoundry-incubator/diego-cf-compatibility) records which versions of cf-release and diego-release are compatible, according to the Diego team's [automated testing pipeline](https://concourse.diego-ci.cf-app.com/?groups=diego).
  - [Diego's Pivotal Tracker project](https://www.pivotaltracker.com/n/projects/1003146) shows what we're working on these days.

[Lattice](http://lattice.cf) is an easy-to-deploy distribution of Diego designed for experimentation with the next-generation core of Cloud Foundry.

### Table of Contents
1. [Discovering a Set of Releases to Deploy](#release-compatibility)
1. [Deploying Diego to BOSH-Lite](#deploying-diego-to-bosh-lite)
1. [Pushing to Diego](#pushing-to-diego)
1. [Deploying Diego to AWS](#deploying-diego-to-aws)
1. [Database Encryption](#database-encryption)
  1. [Configuring Encryption Keys](#configuring-encryption-keys)
1. [TLS Configuration](#tls-configuration)
  1. [Generating TLS Certificates](#generating-tls-certificates)
  1. [Custom TLS Certificate Generation](#custom-tls-certificate-generation)
1. [BOSH Dependencies](#bosh-dependencies)
1. [Recommended Instance Types](#recommended-instance-types)

---

## <a name="compatibility"></a>Release Compatibility

Diego releases are tested against Cloud Foundry, Garden, and ETCD. Compatible versions
of Garden and ETCD are listed with Diego on the [Github releases page](https://github.com/cloudfoundry-incubator/diego-release/releases).

### Checking out a release of Diego

The Diego git repository is tagged with every release. To move the git repository
to match a release, do the following:

```bash
cd diego-release/
# checking out release v0.1437.0
git checkout v0.1437.0
./scripts/update
git clean -ffd
```

### From a final release of CF

On the CF Release [GitHub Releases](https://github.com/cloudfoundry/cf-release/releases) page,
recommended versions of Diego, Garden, and ETCD are listed with each CF Release.
This is the easiest way to correlate releases.

Alternatively, you can use records of CF and Diego compatibility captured from
automated testing. First look up the release candidate SHA for your CF release.
This is listed as the `commit_hash` in the release yaml file. Find the SHA in
[diego-cf-compatibility/compatibility-v2.csv](https://github.com/cloudfoundry-incubator/diego-cf-compatibility/blob/master/compatibility-v2.csv)
to look up tested versions of Diego Release, Garden, and ETCD.

Example: Let's say you want to deploy Diego alongside CF final release `222`. The release file
[`releases/cf-222.yml`](https://github.com/cloudfoundry/cf-release/blob/master/releases/cf-222.yml)
in the cf-release repository contains the line `commit_hash: 53014242`.
Finding `53014242` in `diego-cf-compatibility/compatibility-v2.csv` reveals Diego
0.1437.0, Garden 0.308.0, and ETCD 16 have been verified to be compatible.


### From a specific CF Release commit SHA

Not every cf-release commit will appear in the diego-cf compatibility table,
but many will work with some version of Diego.

If you can't find a specific cf-release SHA in the table, deploy the diego-release
that matches the most recent cf-release relative to that commit. To do this, go back
through cf-release's git log from your commit until you find a Final Release commit
and then look up that commit's SHA in the diego-cf compatibility table.

## Deploying Diego to BOSH-Lite

1. Install and start [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite),
   following its
   [README](https://github.com/cloudfoundry/bosh-lite/blob/master/README.md).
   For garden-linux to function properly in the Diego deployment,
   we recommend using version 9000.69.0 or later of the BOSH-Lite Vagrant box image.

1. Upload the latest version of the Warden BOSH-Lite stemcell directly to BOSH-Lite:

        bosh upload stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent

    Alternately, download the stemcell locally first and then upload it to BOSH-Lite:

        curl -L -o bosh-lite-stemcell-latest.tgz https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent
        bosh upload stemcell bosh-lite-stemcell-latest.tgz

    Please note that the consul_agent job does not set up DNS correctly on version 3126 of the Warden BOSH-Lite stemcell, so we do not recommend the use of that stemcell version.

1. Check out cf-release (runtime-passed branch or tagged release) from git:

        cd ~/workspace
        git clone https://github.com/cloudfoundry/cf-release.git
        cd ~/workspace/cf-release
        git checkout runtime-passed # do not push to runtime-passed
        ./scripts/update

1. Check out diego-release (master branch or tagged release) from git:

        cd ~/workspace
        git clone https://github.com/cloudfoundry-incubator/diego-release.git
        cd ~/workspace/diego-release
        git checkout master # do not push to master
        ./scripts/update

1. Install `spiff` according to its [README](https://github.com/cloudfoundry-incubator/spiff).
   `spiff` is a tool for generating BOSH manifests that is required in some of the scripts used below.

1. Generate the CF manifest:

        cd ~/workspace/cf-release
        ./scripts/generate-bosh-lite-dev-manifest

   **Or if you are running Windows cells** along side this deployment, instead generate the CF manifest as follows:

        cd ~/workspace/cf-release
        ./scripts/generate-bosh-lite-dev-manifest \
          ~/workspace/diego-release/manifest-generation/stubs-for-cf-release/enable_diego_windows_in_cc.yml

1. Generate the Diego manifests:

        cd ~/workspace/diego-release
        ./scripts/generate-bosh-lite-manifests

1. Create, upload, and deploy the CF release:

        cd ~/workspace/cf-release
        bosh deployment bosh-lite/deployments/cf.yml
        bosh create release --name diego --force &&
        bosh -n upload release &&
        bosh -n deploy

1. Upload the latest garden-linux-release:

        bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-linux-release

  If you wish to upload a specific version of garden-linux-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry-incubator/garden-linux-release).

1. Upload the latest etcd-release:

        bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release

  If you wish to upload a specific version of etcd-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry-incubator/etcd-release).

1. Create, upload, and deploy the Diego release:

        cd ~/workspace/diego-release
        bosh deployment bosh-lite/deployments/diego.yml
        bosh create release --force &&
        bosh -n upload release &&
        bosh -n deploy

1. Login to CF and enable Docker support:

        cf login -a api.bosh-lite.com -u admin -p admin --skip-ssl-validation &&
        cf enable-feature-flag diego_docker

Now you are configured to push an app to the BOSH-Lite deployment, or to run the
[Diego Smoke Tests](https://github.com/cloudfoundry-incubator/diego-smoke-tests)
or the
[CF Acceptance Tests](https://github.com/cloudfoundry/cf-acceptance-tests).

> If you wish to run all of the diego jobs on a single VM, you can replace the
> `manifest-generation/bosh-lite-stubs/instance-count-overrides.yml` stub with
> the `manifest-generation/bosh-lite-stubs/colocated-instance-count-overrides.yml`
> stub.

## Pushing a CF Application to the Diego backend

1. Create and target a CF org and space:

        cf api --skip-ssl-validation api.bosh-lite.com
        cf auth admin admin
        cf create-org diego
        cf target -o diego
        cf create-space diego
        cf target -s diego

1. Change into your application directory and push your application without starting it:

        cd <app-directory>
        cf push my-app --no-start

1. [Enable Diego](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/migrating-to-diego.md#targeting-diego) for your application.

1. Start your application:

        cf start my-app

---

##<a name="deploying-diego-to-aws"></a>Deploying Diego to AWS

In order to deploy Diego to AWS follow [these instructions](examples/aws/README.md). Enjoy!

---
## Database Encryption

Diego Release must be configured with a set of encryption keys to be used when
encrypting data at rest in the ETCD database. To configure encryption the
`diego.bbs.encryption_keys` and `diego.bbs.active_key_label` properties should
be set.

Diego will automatically (re-)encrypt all of the data stored in ETCD using the
active key upon boot. This ensures an operator can rotate a key out without
having to manually rewrite all of the records.

### Configuring Encryption Keys

Diego uses multiple keys for decryption while allowing only one for encryption.
This allows an operator to rotate encryption keys in a downtime-less way.

For example:

```yaml
properties:
  diego:
    bbs:
      active_key_label: key-2015-09
      encryption_keys:
        - label: 'key-2015-09'
          passphrase: 'my september passphrase'
        - label: 'key-2015-08'
          passphrase: 'my august passphrase'
```

In the above, the operator is configuring two encryption, and selecting one to
be the active. The active key is the one used for encryption while all the
other can be used for decryption.

The key labels must be no longer than 127 characters, while the passphrases
have no enforced limit. In addtion to that, the key label must not contain a
`:` (colon) character, due the way we build command line flags using `:` as a
separator.

---
##<a name="tls-configuration"></a>TLS Configuration

Diego Release can be configured to require TLS for communication with etcd.
To enable or disable TLS communication with etcd, the `diego.etcd.require_ssl`
and `diego.<component>.etcd.require_ssl` properties should be set to `true` or
`false`.  By default, Diego has `require_ssl` set to `true`.  When
`require_ssl` is `true`, the operator must generate TLS certificates and keys
for the etcd server and its clients.

TLS and mutual authentication can also be enabled between etcd peers. To
enable or disable this, the `diego.etcd.peer_require_ssl` property should be
set to `true` or `false`. By default, Diego has `peer_require_ssl` set to
`true`.  When `peer_require_ssl` is set to `true`, the operator must provide
TLS certificates and keys for the cluster members. The CA, server certificate,
and server key across may be shared between the client and peer configurations
if desired.

Similarly, TLS with mutual authentication can be enabled for communication to
the BBS server, via the `diego.bbs.require_ssl` BOSH property, which defaults
to `true`. When enabled, the operator must provide TLS certificates and keys
for the BBS server and its clients (other components in the Diego deployment).


### Generating TLS Certificates

For generating TLS certificates, we recommend
[certstrap](https://github.com/square/certstrap).  An operator can follow the
following steps to successfully generate the required certificates.

> Most of these commands can be found in
> [scripts/generate-diego-ca-certs](scripts/generate-diego-ca-certs),
> [scripts/generate-etcd-certs](scripts/generate-etcd-certs), and
> [scripts/generate-bbs-certs](scripts/generate-bbs-certs)


1. Get certstrap
   ```
   go get github.com/square/certstrap
   cd $GOPATH/src/github.com/square/certstrap
   ./build
   cd bin
   ```

2. Initialize a new certificate authority.
   ```
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
   ```
   $ ./certstrap request-cert --common-name "etcd.service.cf.internal" --domain "*.etcd.service.cf.internal,etcd.service.cf.internal"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/etcd.service.cf.internal.key
   Created out/etcd.service.cf.internal.csr

   $ ./certstrap sign etcd.service.cf.internal --CA diegoCA
   Created out/etcd.service.cf.internal.crt from out/etcd.service.cf.internal.csr signed by out/diegoCA.key
   ```

   The manifest property `properties.diego.etcd.server_cert` should be set to the certificate in `out/etcd.service.cf.internal.crt`.
   The manifest property `properties.diego.etcd.server_key` should be set to the certificate in `out/etcd.service.cf.internal.key`.

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

   The manifest property `properties.diego.etcd.client_cert` should be set to the certificate in `out/clientName.crt`.
   The manifest property `properties.diego.etcd.client_key` should be set to the certificate in `out/clientName.key`.

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

6. Create and sign a certificate for bbs clients.
   ```
   $ ./certstrap request-cert --common-name "clientName"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created out/clientName.key
   Created out/clientName.csr

   $ ./certstrap sign clientName --CA diegoCA
   Created out/clientName.crt from out/clientName.csr signed by out/diegoCA.key
   ```

   The manifest property `properties.diego.CLIENT.bbs.client_cert` should be set to the certificate in `out/clientName.crt`,
   and the manifest property `properties.diego.CLIENT.bbs.client_key` should be set to the certificate in `out/clientName.key`,
   Where `CLIENT` is each of the diego components that has a BBS client.

7. (Optional) Initialize a new peer certificate authority.
   ```
   $ ./certstrap --depot-path peer init --common-name "peerCA"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created peer/peerCA.key
   Created peer/peerCA.crt
   ```

   The manifest property `properties.diego.etcd.peer_ca_cert` should be set to the certificate in `peer/peerCA.crt`.

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

   The manifest property `properties.diego.etcd.peer_cert` should be set to the certificate in `peer/etcd.service.cf.internal.crt`.
   The manifest property `properties.diego.etcd.peer_key` should be set to the certificate in `peer/etcd.service.cf.internal.key`.


### Custom TLS Certificate Generation

If you already have a CA, or wish to use your own names for clients and
servers, please note that the common-names "diegoCA" and "clientName" are
placeholders and can be renamed provided that all clients client certificate.
The server certificate must have the common name `etcd.service.cf.internal` and
must specify `etcd.service.cf.internal` and `*.etcd.service.cf.internal` as
Subject Alternative Names (SANs).

---
## BOSH Dependencies

When deploying diego-release to a BOSH director you should have at least:

* BOSH Release v206+ (1.3072.0)
* BOSH Stemcell 3125+

---
## Recommended Instance Types

If you are deploying to AWS, you can use our recommended instance types by spiff merging
your `iaas-settings.yml` with our provided `manifest-generation/misc-templates/aws-iaas-settings.yml`:

```
  spiff merge \
    manifest-generation/misc-templates/aws-iaas-settings.yml \
    /path/to/iaas-settings.yml \
    > /tmp/iaas-settings.yml
```

You can then use the template generated as the `iaas-settings.yml` for the `scripts/generate-deployment-manifest` tool.
The cell jobs currently use `r3.xlarge` as their `instance_type`.
For production deployments, we recommend that you increase the `ephemeral_disk` size.
This can be done by specifying the following in your `iaas-settings.yml` under the cell resource pool definitions:

```
ephemeral_disk:
  size: 174_080
  type: gp2
```
