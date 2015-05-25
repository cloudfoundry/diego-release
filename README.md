# Cloud Foundry Diego [BOSH release]

----
This repo is a [BOSH](https://github.com/cloudfoundry/bosh) release for deploying Diego
and associated tasks for testing a Diego deployment.  Diego builds out the new runtime
architecture for Cloud Foundry, replacing the DEAs and Health Manager.

This release relies on a separate deployment to provide
[NATS](https://github.com/apcera/gnatsd) and
[Loggregator](https://github.com/cloudfoundry/loggregator). In practice these
come from [cf-release](https://github.com/cloudfoundry/cf-release).

**Learn more about Diego and its components at
[diego-design-notes](https://github.com/cloudfoundry-incubator/diego-design-notes).**

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
        curl -L -o $TMPDIR/consul-0.5.0.zip "https://dl.bintray.com/mitchellh/consul/0.5.0_${os}_amd64.zip"
        unzip $TMPDIR/consul-0.5.0.zip -d ~/workspace/diego-release/bin
        rm $TMPDIR/consul-0.5.0.zip

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
        git clone git@github.com:cloudfoundry/cf-release.git
        cd ~/workspace/cf-release
        ./update

1. Checkout diego-release (develop branch) from git

        cd ~/workspace
        git clone git@github.com:cloudfoundry-incubator/diego-release.git
        cd ~/workspace/diego-release
        git checkout develop
        ./scripts/update

1. Install spiff, a tool for generating BOSH manifests. spiff is required for
   running the scripts in later steps. The following installation method
   assumes that go is installed. For other ways of installing `spiff`, see
   [the spiff README](https://github.com/cloudfoundry-incubator/spiff).

        go get github.com/cloudfoundry-incubator/spiff

1. Generate a deployment stub with the BOSH director UUID

        mkdir -p ~/deployments/bosh-lite
        cd ~/workspace/diego-release
        ./scripts/print-director-stub > ~/deployments/bosh-lite/director.yml

1. Generate and target cf-release manifest:

        cd ~/workspace/cf-release
        ./generate_deployment_manifest warden \
            ~/deployments/bosh-lite/director.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_diego_docker_in_cc.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_consul_with_cf.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_diego_ssh_in_cc.yml \
            ~/workspace/diego-release/manifest-generation/bosh-lite-stubs/property-overrides.yml \
            > ~/deployments/bosh-lite/cf.yml
        bosh deployment ~/deployments/bosh-lite/cf.yml

   **Or if you are running Windows cells** along side this deployment, instead generate cf-release manifest using:

        cd ~/workspace/cf-release
        ./generate_deployment_manifest warden \
            ~/deployments/bosh-lite/director.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_diego_docker_in_cc.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_consul_with_cf.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_diego_windows_in_cc.yml \
            ~/workspace/diego-release/stubs-for-cf-release/enable_diego_ssh_in_cc.yml \
            ~/workspace/diego-release/manifest-generation/bosh-lite-stubs/property-overrides.yml \
            > ~/deployments/bosh-lite/cf.yml
        bosh deployment ~/deployments/bosh-lite/cf.yml

1. Do the BOSH dance:

        cd ~/workspace/cf-release
        bosh create release --force
        bosh -n upload release
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

1. Dance some more:

        bosh create release --force
        bosh -n upload release
        bosh -n deploy

Now you can either run the DATs or deploy your own app.

---
###<a name="smokes-and-dats"></a> Running Smoke Tests & DATs

You can test that your diego-release deployment is working and integrating with cf-release
by running the lightweight `diego-smoke-tests` or the more thorough `diego-acceptance-tests`.
Follow the READMEs in their respective repositories:

- `src/github.com/cloudfoundry-incubator/diego-smoke-tests`
- `src/github.com/cloudfoundry-incubator/diego-acceptance-tests`

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

