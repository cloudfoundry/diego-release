# diego release

<p align="center">
  <img src="http://i.imgur.com/WrqaOd9.png" alt="Go Diego Go!" title="Go Diego Go!"/>
</p>

#### Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry-incubator/diego-design-notes)

A [BOSH](https://github.com/cloudfoundry/bosh) release for deploying the
following Diego components:

1. [Executor](https://github.com/cloudfoundry-incubator/executor)
1. [Garden-Linux](https://github.com/cloudfoundry-incubator/garden-linux)
1. [Stager](https://github.com/cloudfoundry-incubator/stager)
1. [File Server](https://github.com/cloudfoundry-incubator/file-server)
1. [Runtime Metrics Server](https://github.com/cloudfoundry-incubator/runtime-metrics-server)
1. [etcd](https://github.com/coreos/etcd)

These components build out the new runtime architecture for Cloud Foundry,
replacing the DEA and Health Manager.

This release relies on a separate deployment to provide
[NATS](https://github.com/apcera/gnatsd) and
[Loggregator](https://github.com/cloudfoundry/loggregator). In practice these
come from [cf-release](https://github.com/cloudfoundry/cf-release).


## Developer Workflow

Work out of the submodules under `src/`. See [Initial Setup](#initial-setup).

Run the individual component unit tests as you work on them. To see if
*everything* still works, run `./scripts/run_unit_tests` in the root of the
release.

When you're ready to commit, run:

    ```bash
    ./scripts/preparetodiego
    ```

This will synchronize submodules, update the BOSH package specs, run all unit
tests, all integration tests, and make a commit, bringing up a commit edit
dialogue.

If you're introducing a new component (e.g. a new job/errand), make sure it
has been added to `./scripts/sync-package-specs` and
`./scripts/sync-submodule-config`.


## Initial Setup

This BOSH release doubles as a `$GOPATH`. It will automatically be set up for
you if you have [direnv](http://direnv.net) installed.

```bash
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
```


## Running Unit Tests

1. Install ginkgo
   ```sh
   go install github.com/onsi/ginkgo/ginkgo
   ```

1. Run the unit test script
   ```sh
   ./scripts/run_unit_tests
   ```


## Running Integration Tests

1. Install and start [Concourse](http://concourse.ci), following its
   [README](https://github.com/concourse/concourse/blob/master/README.md).

1. Install the `fly` CLI:

    ```sh
    # cd to the concourse release repo
    cd concourse/

    # install the version of fly from Concourse's release
    go install github.com/concourse/fly

    # add the concourse release repo's bin/ directory to your $PATH
    export PATH=$PWD/bin:$PATH
    ```

1. Run [Inigo](https://github.com/cloudfoundry-incubator/inigo).

    ```sh
    ./scripts/run_inigo
    ```


## Deploying Diego to a local Bosh-Lite instance

1. Install and start [BOSH Lite](https://github.com/cloudfoundry/bosh-lite),
   following its
   [README](https://github.com/cloudfoundry/bosh-lite/blob/master/README.md).

1. Download the latest Warden Trusty Go-Agent stemcell and upload it to bosh-lite

  ```bash
  bosh public stemcells
  bosh download public stemcell (name)
  bosh upload stemcell (downloaded filename)
  ```

1. Checkout cf-release (develop branch) from git

  ```bash
  cd ~/workspace
  git clone git@github.com:cloudfoundry/cf-release.git
  cd ~/workspace/cf-release
  git checkout develop
  ./update
  ```

1. Checkout diego-release (develop branch) from git

  ```bash
  cd ~/workspace
  git clone git@github.com:cloudfoundry-incubator/diego-release.git
  cd ~/workspace/diego-release
  git checkout develop
  ./scripts/update
  ```

1. Install spiff, a tool for generating bosh manifests. spiff is required for
   running the scripts in later steps. The following installation method
   assumes that go is installed. For other ways of installing `spiff`, see
   [the spiff README](https://github.com/cloudfoundry-incubator/spiff).

  ```bash
  go get github.com/cloudfoundry-incubator/spiff
  ```

1. Generate a deployment stub with the bosh director uuid

  ```bash
  mkdir -p ~/deployments/bosh-lite
  scripts/generate_director_stub > ~/deployments/bosh-lite/director.yml
  ```

1. Generate and target cf-release manifest:
  ```bash
  cd ~/workspace/cf-release
  ./generate_deployment_manifest warden \
      ~/deployments/bosh-lite/director.yml \
      ~/workspace/diego-release/templates/enable_diego_in_cc.yml > \
      ~/deployments/bosh-lite/cf.yml
  bosh deployment ~/deployments/bosh-lite/cf.yml
  ```

1. Do the bosh dance:
  ```bash
  cd ~/workspace/cf-release
  bosh create release --force
  bosh -n upload release
  bosh -n deploy
  ```

1. Generate and target diego's manifest:

  ```bash
  cd ~/workspace/diego-release
  ./generate_deployment_manifest bosh-lite ../cf-release \
      ~/deployments/bosh-lite/director.yml > \
      ~/deployments/bosh-lite/diego.yml
  bosh deployment ~/deployments/bosh-lite/diego.yml
  ```

1. Dance some more:

  ```bash
  bosh create release --force
  bosh -n upload release
  bosh -n deploy
  ```

Now you can either run the CATs or deploy your own app.

### Running the CATs & DATs

These can both be run as BOSH errands:

```
bosh -d ~/deployments/bosh-lite/cf.yml run errand acceptance_tests
bosh -d ~/deployments/bosh-lite/diego.yml run errand diego_acceptance_tests
```

### Pushing an Application to Diego

1. Create new CF Org & Space

  ```
  cf api --skip-ssl-validation api.10.244.0.34.xip.io
  cf auth admin admin
  cf create-org diego
  cf target -o diego
  cf create-space diego
  cf target -s diego
  ```

1. Push your application

  ```
  cf push my-app --no-start
  cf set-env my-app CF_DIEGO_BETA true
  cf set-env my-app CF_DIEGO_RUN_BETA true
  cf start my-app
  ```

  The `CF_DIEGO_BETA` flag instructs the cloud controller to stage the application on Diego.  `CF_DIEGO_RUN_BETA` instructs the cloud controller to run the application on Diego.  While apps that run on Diego *must* stage on Diego, you can experiment with *staging* an app on Diego but running it on the DEAs.  Simply skip specifying `CF_DIEGO_RUN_BETA`.
