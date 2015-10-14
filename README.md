# Cloud Foundry Diego [BOSH release]

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

  - The [Diego Design Notes](https://github.com/cloudfoundry-incubator/diego-design-notes) present an overview of Diego, and links to the various Diego components.
  - The [Migration Guide](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/migrating-to-diego.md) describes how developers and operators can manage a transition from the DEAs to Diego.
  - The [Docker Support Notes](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/docker-support.md) describe how Diego runs Docker-image-based apps in Cloud Foundry.
  - The [SSH Access Notes](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/ssh-access-and-policy.md) describe how to use the Diego-SSH CLI plugin to connect to app instances running on Diego.
  - The [Diego-CF Compatibility Log](https://github.com/cloudfoundry-incubator/diego-cf-compatibility) records which versions of cf-release and diego-release are compatible, according to the Diego team's [automated testing pipeline](https://concourse.diego-ci.cf-app.com/?groups=diego).
  - [Diego's Pivotal Tracker project](https://www.pivotaltracker.com/n/projects/1003146) shows what we're working on these days.

[Lattice](http://lattice.cf) is an easy-to-deploy distribution of Diego designed for experimentation with the next-generation core of Cloud Foundry.

### Table of Contents
1. [Developer Workflow](#developer-workflow)
1. [Initial Setup](#initial-setup)
1. [Deploying Diego to BOSH-Lite](#deploying-diego-to-bosh-lite)
1. [Pushing to Diego](#pushing-to-diego)
1. [Testing Diego](#testing-diego)
  1. [Running Unit Tests](#running-unit-tests)
  1. [Running Integration Tests](#running-integration-tests)
  1. [Running Benchmark Tests](#running-benchmark-tests)
  1. [Running Smoke Tests & DATs](#smokes-and-dats)
1. [Database Encryption](#database-encryption)
  1. [Configuring Encryption Keys](#configuring-encryption-keys)
1. [TLS Configuration](#tls-configuration)
  1. [Generating TLS Certificates](#generating-tls-certificates)
  1. [Custom TLS Certificate Generation](#custom-tls-certificate-generation)
1. [Recommended Instance Types](#recommended-instance-types)

----
## Developer Workflow

When working on individual components of Diego, work out of the submodules under `src/`.
See [Initial Setup](#initial-setup).

Run the individual component unit tests as you work on them using
[ginkgo](https://github.com/onsi/ginkgo). To see if *everything* still works, run
`./scripts/run-unit-tests` in the root of the release.

When you're ready to commit, run:

    ./scripts/prepare-to-diego <story-id> <another-story-id>...

This will synchronize submodules, update the BOSH package specs, run all unit
tests, all integration tests, and make a commit, bringing up a commit edit
dialogue.  The story IDs correspond to stories in our [Pivotal Tracker
backlog](https://www.pivotaltracker.com/n/projects/1003146).  You should
simultaneously also build the release and deploy it to a local
[BOSH-Lite](https://github.com/cloudfoundry/bosh-lite) environment, and run the
acceptance tests.  See [Running Smoke Tests & DATs](#smokes-and-dats).

If you're introducing a new component (e.g. a new job/errand) or changing the main path
for an existing component, make sure to update `./scripts/sync-package-specs` and
`./scripts/sync-submodule-config`.

---
##Initial Setup

This BOSH release doubles as a `$GOPATH`. It will automatically be set up for
you if you have [direnv](http://direnv.net) installed.

    # fetch release repo
    mkdir -p ~/workspace
    cd ~/workspace

    # fetch garden-linux-release
    git clone https://github.com/cloudfoundry-incubator/garden-linux-release.git
    (cd garden-linux-release/ && git checkout master && git submodule update --init --recursive)

    git clone https://github.com/cloudfoundry-incubator/diego-release.git
    cd diego-release/

    # automate $GOPATH and $PATH setup
    direnv allow

    # switch to develop branch (not master!)
    git checkout develop

    # initialize and sync submodules
    ./scripts/update

If you do not wish to use direnv, you can simply `source` the `.envrc` file in the root
of the release repo.  You may manually need to update your `$GOPATH` and `$PATH` variables
as you switch in and out of the directory.

---
## Deploying Diego to BOSH-Lite

1. Install and start [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite),
   following its
   [README](https://github.com/cloudfoundry/bosh-lite/blob/master/README.md).

1. Download the latest Warden Trusty Go-Agent stemcell and upload it to BOSH-lite:

        bosh public stemcells
        bosh download public stemcell (name)
        bosh upload stemcell (downloaded filename)

1. Check out cf-release (develop branch) from git:

        cd ~/workspace
        git clone https://github.com/cloudfoundry/cf-release.git
        cd ~/workspace/cf-release
        git checkout develop
        ./scripts/update

1. Check out diego-release (develop branch) from git:

        cd ~/workspace
        git clone https://github.com/cloudfoundry-incubator/diego-release.git
        cd ~/workspace/diego-release
        git checkout develop
        ./scripts/update

1. Install `spiff` according to its [README](https://github.com/cloudfoundry-incubator/spiff).
   `spiff` is a tool for generating BOSH manifests that is required in some of the scripts used below.

1. Generate the CF manifest:

        cd ~/workspace/cf-release
        ./scripts/generate-bosh-lite-dev-manifest

   **Or if you are running Windows cells** along side this deployment, instead generate the CF manifest as follows:

        cd ~/workspace/cf-release
        ./scripts/generate-bosh-lite-dev-manifest \
          ~/workspace/diego-release/stubs-for-cf-release/enable_diego_windows_in_cc.yml

1. Generate the Diego manifests:

        cd ~/workspace/diego-release
        ./scripts/generate-bosh-lite-manifests

1. Create, upload, and deploy the CF release:

        cd ~/workspace/cf-release
        bosh deployment bosh-lite/deployments/cf.yml
        bosh create release --force &&
        bosh -n upload release &&
        bosh -n deploy

1. Upload the latest garden-linux-release:

        bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-linux-release

1. Upload the latest etcd-release:

        bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release

1. Create, upload, and deploy the Diego release:

        cd ~/workspace/diego-release
        bosh deployment bosh-lite/deployments/diego.yml
        bosh create release --force
        bosh -n upload release &&
        bosh -n deploy &&

1. Login to CF and enable Docker support:

        cf login -a api.bosh-lite.com -u admin -p admin --skip-ssl-validation &&
        cf enable-feature-flag diego_docker

Now you are configured to push an app to the BOSH-Lite deployment, or to run the
[Diego Smoke Tests](https://github.com/cloudfoundry-incubator/diego-smoke-tests)
or the
[Diego Acceptance Tests](https://github.com/cloudfoundry-incubator/diego-acceptance-tests).

> If you wish to run all of the diego jobs on a single VM, you can replace the
> `manifest-generation/bosh-lite-stubs/instance-count-overrides.yml` stub with
> the `manifest-generation/bosh-lite-stubs/colocated-instance-count-overrides.yml`
> stub.


---
## Pushing to Diego

1. Create new CF Org & Space:

        cf api --skip-ssl-validation api.bosh-lite.com
        cf auth admin admin
        cf create-org diego
        cf target -o diego
        cf create-space diego
        cf target -s diego

1. Push your application without starting it:

        cf push my-app --no-start

1. [Enable Diego](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/migrating-to-diego.md#targeting-diego) for your application.

1. Start your application:

        cf start my-app


---
## Testing Diego

### Running Unit Tests

1. Install ginkgo

        go install github.com/onsi/ginkgo/ginkgo

2. Install gnatsd

        go install github.com/apcera/gnatsd

3. Install etcd

        go install github.com/coreos/etcd

4. Install consul

        if uname -a | grep Darwin; then os=darwin; else os=linux; fi
        curl -L -o $TMPDIR/consul-0.5.2.zip "https://dl.bintray.com/mitchellh/consul/0.5.2_${os}_amd64.zip"
        unzip $TMPDIR/consul-0.5.2.zip -d ~/workspace/diego-release/bin
        rm $TMPDIR/consul-0.5.2.zip

5. Run the unit test script

        ./scripts/run-unit-tests


### Running Integration Tests

1. Install and start [Concourse](http://concourse.ci), following its
   [README](https://github.com/concourse/concourse/blob/master/README.md).

1. Install the `fly` CLI:

        # cd to the concourse release repo,
        cd /path/to/concourse/repo

        # switch to using the concourse $GOPATH and $PATH setup temporarily
        direnv allow

        # install the version of fly from Concourse's release
        go install github.com/concourse/fly

        # add the concourse release repo's bin/ directory to your $PATH
        export PATH=$PWD/bin:$PATH

1. Run [Inigo](https://github.com/cloudfoundry-incubator/inigo).

        # cd back to the diego-release release repo
        cd diego-release/

        # run the tests
        ./scripts/run-inigo

### Running Benchmark Tests

WARNING: Benchmark tests drop the database.

1. Deploy diego-release to an environment (use instance-count-overrides to turn 
   off all components except the database for a cleaner test)

1. Depending on whether you're deploying to AWS or bosh-lite, copy either 
   `manifest-generation/benchmark-errand-stubs/defaut_aws_benchmark_properties.yml` or 
   `manifest-generation/benchmark-errand-stubs/defaut_bosh_lite_benchmark_properties.yml` 
   to your local deployments or stubs folder and fill it in.

1. Generate a benchmark deployment manifest using 
   `./scripts/generate-benchmarks-manifest /path/to/diego.yml /path/to/benchmark-properties.yml > benchmark.yml`

1. Deploy and run the tests using 
   `bosh -d benchmark.yml -n deploy && bosh -d benchmark.yml -n run errand benchmark-bbs`


###<a name="smokes-and-dats"></a> Running Smoke Tests & DATs

You can test that your diego-release deployment is working and integrating with
cf-release by running the lightweight
[diego-smoke-tests](https://github.com/cloudfoundry-incubator/diego-smoke-tests)
or the more thorough
[diego-acceptance-tests](https://github.com/cloudfoundry-incubator/diego-acceptance-tests).


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
## TLS Configuration

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
