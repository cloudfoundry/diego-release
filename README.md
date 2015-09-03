# Cloud Foundry Diego [BOSH release]

----
This repo is a [BOSH](https://github.com/cloudfoundry/bosh) release for deploying Diego
and associated tasks for testing a Diego deployment.  Diego builds out the new runtime
architecture for Cloud Foundry, replacing the DEAs and Health Manager.

This release relies on a separate deployment to provide [Consul](https://github.com/hashicorp/consul),
[NATS](https://github.com/apcera/gnatsd) and
[Loggregator](https://github.com/cloudfoundry/loggregator). In practice these
come from [cf-release](https://github.com/cloudfoundry/cf-release).

Additional Diego resources:

  - The [Diego Design Notes](https://github.com/cloudfoundry-incubator/diego-design-notes) present an overview of Diego, and links to the various Diego components.
  - The [Receptor API Docs](https://github.com/cloudfoundry-incubator/receptor/tree/master/doc) describe the public API to  Diego, which clients such as CF's Cloud Controller and the Lattice CLI use to run workloads on Diego.
  - The [Migration Guide](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/migrating-to-diego.md) describes how developers and operators can manage a transition from the DEAs to Diego.
  - The [Docker Support Notes](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/docker-support.md) describe how Diego runs Docker-image-based apps in Cloud Foundry.
  - The [SSH Access Notes](https://github.com/cloudfoundry-incubator/diego-design-notes/blob/master/ssh-access-and-policy.md) describe how to use the Diego-SSH CLI plugin to connect to app instances running on Diego.
  - The [Diego-CF Compatibility Log](https://github.com/cloudfoundry-incubator/diego-cf-compatibility) records which versions of cf-release and diego-release are compatible, according to the Diego team's [automated testing pipeline](https://concourse.diego-ci.cf-app.com/?groups=diego).
  - [Diego's Pivotal Tracker project](https://www.pivotaltracker.com/n/projects/1003146) shows what we're working on these days.

[Lattice](http://lattice.cf) is an easy-to-deploy distribution of Diego designed for experimentation with the next-generation core of Cloud Foundry.

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
dialogue.  The story IDs correspond to stories in our
[Pivotal Tracker backlog](https://www.pivotaltracker.com/n/projects/1003146).
You should simultaneously also build the release and deploy it to a local
[BOSH-Lite](https://github.com/cloudfoundry/bosh-lite) environment, and run the acceptance
tests.  See [Running Smoke Tests & DATs](#smokes-and-dats).

If you're introducing a new component (e.g. a new job/errand) or changing the main path
for an existing component, make sure to update `./scripts/sync-package-specs` and
`./scripts/sync-submodule-config`.

---
##<a name="initial-setup"></a> Initial Setup

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
## Running Unit Tests

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


---
## Running Integration Tests

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

---

## Deploying Diego to a local BOSH-Lite instance

1. Install and start [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite),
   following its
   [README](https://github.com/cloudfoundry/bosh-lite/blob/master/README.md).

1. Download the latest Warden Trusty Go-Agent stemcell and upload it to BOSH-lite

        bosh public stemcells
        bosh download public stemcell (name)
        bosh upload stemcell (downloaded filename)

1. Checkout cf-release (develop branch) from git

        cd ~/workspace
        git clone https://github.com/cloudfoundry/cf-release.git
        cd ~/workspace/cf-release
        git checkout develop
        ./scripts/update

1. Checkout diego-release (develop branch) from git

        cd ~/workspace
        git clone https://github.com/cloudfoundry-incubator/diego-release.git
        cd ~/workspace/diego-release
        git checkout develop
        ./scripts/update

1. Install `spiff`, a tool for generating BOSH manifests. `spiff` is required
   for running the scripts in later steps. For instructions on installing 
   `spiff`, see its [README](https://github.com/cloudfoundry-incubator/spiff).

1. Generate a deployment stub with the BOSH director UUID

        mkdir -p ~/deployments/bosh-lite
        cd ~/workspace/diego-release
        ./scripts/print-director-stub > ~/deployments/bosh-lite/director.yml

1. Generate and target cf-release manifest:

        cd ~/workspace/cf-release
        ./scripts/generate_deployment_manifest warden \
            ~/deployments/bosh-lite/director.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_consul_with_cf.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_diego_ssh_in_cf.yml \
            > ~/deployments/bosh-lite/cf.yml
        bosh deployment ~/deployments/bosh-lite/cf.yml

   **Or if you are running Windows cells** along side this deployment, instead generate cf-release manifest using:

        cd ~/workspace/cf-release
        ./scripts/generate_deployment_manifest warden \
            ~/deployments/bosh-lite/director.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_consul_with_cf.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_diego_windows_in_cc.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_diego_ssh_in_cf.yml \
            > ~/deployments/bosh-lite/cf.yml
        bosh deployment ~/deployments/bosh-lite/cf.yml

1. Do the BOSH dance:

        cd ~/workspace/cf-release
        bosh create release --force &&
        bosh -n upload release &&
        bosh -n deploy

1. Generate and target diego's manifest:

        cd ~/workspace/diego-release
        ./scripts/generate-deployment-manifest \
            ~/deployments/bosh-lite/director.yml \
            manifest-generation/bosh-lite-stubs/property-overrides.yml \
            manifest-generation/bosh-lite-stubs/instance-count-overrides.yml \
            manifest-generation/bosh-lite-stubs/persistent-disk-overrides.yml \
            manifest-generation/bosh-lite-stubs/iaas-settings.yml \
            manifest-generation/bosh-lite-stubs/additional-jobs.yml \
            ~/deployments/bosh-lite \
            > ~/deployments/bosh-lite/diego.yml
        bosh deployment ~/deployments/bosh-lite/diego.yml

1. Upload the garden-linux-release

        bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-linux-release

1. Dance some more:

        bosh create release --force &&
        bosh -n upload release &&
        bosh -n deploy

1. Login to CF and enable Docker support

        cf login -a api.10.244.0.34.xip.io -u admin -p admin --skip-ssl-validation &&
        cf enable-feature-flag diego_docker

Now you can either run the DATs or deploy your own app.

> If you wish to run all of the diego jobs on a single VM, you can replace the
> `manifest-generation/bosh-lite-stubs/instance-count-overrides.yml` stub with
> the `manifest-generation/bosh-lite-stubs/colocated-instance-count-overrides.yml`
> stub.

---
###<a name="smokes-and-dats"></a> Running Smoke Tests & DATs

You can test that your diego-release deployment is working and integrating with cf-release
by running the lightweight [diego-smoke-tests](https://github.com/cloudfoundry-incubator/diego-smoke-tests) or the more thorough [diego-acceptance-tests](https://github.com/cloudfoundry-incubator/diego-acceptance-tests).

---
### Pushing an Application to Diego

1. Create new CF Org & Space:

        cf api --skip-ssl-validation api.10.244.0.34.xip.io
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


### SSL Configuration

Diego Release can be configured to require SSL for communication with etcd.
To enable or disable SSL communication with etcd, the `diego.etcd.require_ssl`
and `diego.<component>.etcd.require_ssl` properties should be set to `true` or
`false`.  By default, Diego has `require_ssl` set to `true`.  When
`require_ssl` is `true`, the operator must generate SSL certificates and keys
for the etcd server and its clients.

SSL and mutual authentication can also be enabled between etcd peers. To
enable or disable this, the `diego.etcd.peer_require_ssl` property should be
set to `true` or `false`. By default, Diego has `peer_require_ssl` set to
`true`.  When `peer_require_ssl` is set to `true`, the operator must provide
SSL certificates and keys for the cluster members. The CA, server certificate,
and server key across may be shared between the client and peer configurations
if desired.

#### Generating SSL Certificates

For generating SSL certificates, we recommend [certstrap](https://github.com/square/certstrap).
An operator can follow the following steps to successfully generate the required certificates.

> Most of these commands can be found in [scripts/generate-etcd-certs](scripts/generate-etcd-certs)

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

   The manifest property `properties.diego.etcd.ca_cert` should be set to the certificate in `out/diegoCA.crt`

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

   The manifest property `properties.diego.etcd.server_cert` should be set to the certificate in `out/etcd.service.cf.internal.crt`
   The manifest property `properties.diego.etcd.server_key` should be set to the certificate in `out/etcd.service.cf.internal.key`

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

   The manifest property `properties.diego.etcd.client_cert` should be set to the certificate in `out/clientName.crt`
   The manifest property `properties.diego.etcd.client_key` should be set to the certificate in `out/clientName.key`

5. Initialize a new peer certificate authority. [optional]
   ```
   $ ./certstrap --depot-path peer init --common-name "peerCA"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created peer/peerCA.key
   Created peer/peerCA.crt
   ```

   The manifest property `properties.diego.etcd.peer_ca_cert` should be set to the certificate in `peer/peerCA.crt`

6. Create and sign a certificate for the etcd peers. [optional]
   ```
   $ ./certstrap --depot-path peer request-cert --common-name "etcd.service.cf.internal" --domain "*.etcd.service.cf.internal,etcd.service.cf.internal"
   Enter passphrase (empty for no passphrase): <hit enter for no password>

   Enter same passphrase again: <hit enter for no password>

   Created peer/etcd.service.cf.internal.key
   Created peer/etcd.service.cf.internal.csr

   $ ./certstrap --depot-path peer sign etcd.service.cf.internal --CA diegoCA
   Created peer/etcd.service.cf.internal.crt from peer/etcd.service.cf.internal.csr signed by peer/peerCA.key
   ```

   The manifest property `properties.diego.etcd.peer_cert` should be set to the certificate in `peer/etcd.service.cf.internal.crt`
   The manifest property `properties.diego.etcd.peer_key` should be set to the certificate in `peer/etcd.service.cf.internal.key`

#### Custom SSL Certificate Generation

If you already have a CA, or wish to use your own names for clients and
servers, please note that the common-names "diegoCA" and "clientName" are
placeholders and can be renamed provided that all clients client certificate.
The server certificate must have the common name `etcd.service.cf.internal` and
must specify `etcd.service.cf.internal` and `*.etcd.service.cf.internal` as Subject
Alternative Names (SANs).

---
### Recommended Instance Types

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
