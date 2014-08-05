# diego release

<p align="center">
  <img src="http://i.imgur.com/WrqaOd9.png" alt="Go Diego Go!" title="Go Diego Go!"/>
</p>

####Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry-incubator/diego-design-notes)

A [BOSH](https://github.com/cloudfoundry/bosh) release for deploying the
following Diego components:

1. [Executor](https://github.com/cloudfoundry-incubator/executor)
1. [Warden-Linux](https://github.com/cloudfoundry-incubator/warden-linux)
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

## Deploying Diego to a local Bosh-Lite instance

1. checkout bosh-lite from git

  ```bash
  cd ~/workspace
  git clone git@github.com:cloudfoundry/bosh-lite.git
  ```

1. Follow bosh-lite Installation and VMWare Fusion setup steps (requires vmware-fusion license)

  ```bash
  cd ~/workspace/bosh-lite
  vagrant up
  gem install bosh_cli
  bosh target 192.168.50.4
  bosh login admin admin
  ./scripts/add-route
  ```

1. Download the latest Warden stemcell and upload it to bosh-lite

  ```bash
  bosh public stemcells
  bosh download public stemcell bosh-stemcell-3-warden-boshlite-ubuntu-trusty-go_agent.tgz
  bosh upload stemcell bosh-stemcell-3-warden-boshlite-ubuntu-trusty-go_agent.tgz
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

1. Install spiff, a tool for generating bosh manifests. spiff is required for running the scripts in later steps. The following installation method assumes that go is installed. For other ways of installing `spiff`, see [the spiff README](https://github.com/cloudfoundry-incubator/spiff).

  ```bash
  go get github.com/cloudfoundry-incubator/spiff
  ```

1. Generate a deployment stub with the bosh director uuid

  ```bash
  mkdir -p ~/workspace/deployments/warden
  scripts/generate_director_stub > ~/workspace/deployments/warden/director.yml
  ```

1. Generate and target cf-release manifest:
  ```bash
  cd ~/workspace/cf-release
  ./generate_deployment_manifest warden \
      ~/workspace/deployments/warden/director.yml \
      ~/workspace/diego-release/templates/enable_diego_in_cc.yml > \
      ~/workspace/deployments/warden/cf.yml
  bosh deployment ~/workspace/deployments/warden/cf.yml
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
  ./generate_deployment_manifest warden ../cf-release \
      ~/workspace/deployments/warden/director.yml > \
      ~/workspace/deployments/warden/diego.yml
  bosh deployment ~/workspace/deployments/warden/diego.yml
  ```

1. Dance some more:

  ```bash
  bosh create release --force
  bosh -n upload release
  bosh -n deploy
  ```

Now you can either run the CATs or deploy your own app.

### Running the CATs

#### Option 1: Run as a BOSH errand

The CF deployment includes the CATs as the `acceptance_tests` errand, so you can just run them as an errand.

1. Target the CF deployment:
   ```
   bosh deployment ~/workspace/deployments/warden/cf.yml
   ```

2. Run the errand:
   ```
   bosh run errand acceptance_tests
   ```



#### Option 2: Run Locally

If you are making changes to the CATs and want to iterate, you may wish to run the CATs locally.  You'll be running `ginkgo` on your host machine, targetted at your BOSH-lite deployment.

1. Checkout cf-acceptance-tests

  ```bash
  go get -u -v github.com/cloudfoundry/cf-acceptance-tests/...
  cd $GOPATH/src/github.com/cloudfoundry/cf-acceptance-tests
  ```

1. Generate a CATs config file:

  ```bash
  cd $GOPATH/src/github.com/cloudfoundry/cf-acceptance-tests
  cat > integration_config.json <<EOF
  {
    "api": "api.10.244.0.34.xip.io",
    "admin_user": "admin",
    "admin_password": "admin",
    "apps_domain": "10.244.0.34.xip.io",
    "skip_ssl_validation": true,
    "nodes": 1,
    "include_diego": true
  }
  EOF
  export CONFIG=$PWD/integration_config.json
  ```

1. Run the diego CATs:

  ```bash
  cd $GOPATH/src/github.com/cloudfoundry/cf-acceptance-tests
  ginkgo -nodes=4 ./diego
  ```

1. Run the runtime CATs:

  ```bash
  cd $GOPATH/src/github.com/cloudfoundry/cf-acceptance-tests
  ginkgo -nodes=4 ./apps
  ```

### Deploying an app

1. Create new CF Org & Space

  ```
  cf api --skip-ssl-validation api.10.244.0.34.xip.io
  cf auth admin admin
  cf create-org diego
  cf target -o diego
  cf create-space diego
  cf target -s diego
  ```

1. Checkout cf-acceptance-tests (to get, for example, the hello-world app)

  ```bash
  go get -u -v github.com/cloudfoundry/cf-acceptance-tests/...
  cd $GOPATH/src/github.com/cloudfoundry/cf-acceptance-tests/assets/hello-world
  ```

1. Push hello-world app to CF & Configure it to use Diego

  ```
  cf push goodbye --no-start
  cf set-env goodbye CF_DIEGO_BETA true
  cf start goodbye
  ```

### Running an app

Follow the above instructions, but for step 3:

1. Push hello-world app to CF & Configure it to use Diego

  ```
  cf push goodbye --no-start
  cf set-env goodbye CF_DIEGO_BETA true
  cf set-env goodbye CF_DIEGO_RUN_BETA true
  cf push goodbye -i 3 -c ./your/start/command
  ```
