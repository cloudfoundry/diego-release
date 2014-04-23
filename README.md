# diego release

<p align="center">
  <img src="http://i.imgur.com/WrqaOd9.png" alt="Go Diego Go!" title="Go Diego Go!"/>
</p>

A BOSH release for deploying the following Diego components:

1. [Executor](https://github.com/cloudfoundry-incubator/executor)
1. [Warden-Linux](https://github.com/cloudfoundry-incubator/warden-linux)
1. [Stager](https://github.com/cloudfoundry-incubator/stager)
1. [File Server](https://github.com/cloudfoundry-incubator/file-server)
1. [Runtime Metrics Server](https://github.com/cloudfoundry-incubator/runtime-metrics-server)
1. [etcd](https://github.com/coreos/etcd)

These components build out the new runtime architecture for Cloud Foundry,
replacing the DEA and Health Manager.

This release relies on a separate deployment to provide
[NATS](https://github.com/apcera/gnatsd). In practice this comes from
[cf-release](https://github.com/cloudfoundry/cf-release).

## Deploying Diego to a local Bosh-Lite instance

1. checkout bosh-lite from git

  ```bash
  $ cd ~/workspace
  $ git clone git@github.com:cloudfoundry/bosh-lite.git
  $ cd ~/workspace/bosh-lite
  ```

1. Follow bosh-lite Installation and VMWare Fusion setup steps (requires vmware-fusion license)

  ```bash
  install vmware-fusion vagrant plugin
  vagrant plugin license vagrant-vmware-fusion /path/to/license.lic
  vagrant up --provider vmware_fusion
  gem install bosh_cli
  bosh target 192.168.50.4
  bosh login admin admin
  scripts/add-route
  ```

1. Download the latest Warden stemcell and upload it to bosh-lite

  ```bash
  wget http://bosh-jenkins-gems-warden.s3.amazonaws.com/stemcells/latest-bosh-stemcell-warden.tgz
  bosh upload stemcell latest-bosh-stemcell-warden.tgz
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

1. Generate warden-director stub manifest with the bosh director uuid
  
  ```bash
  mkdir -p ~/workspace/deployments/warden
  printf "%s\ndirector_uuid: %s" "---" `bosh status --uuid` > ~/workspace/deployments/warden/director.yml
  ```
 
1. Generate the combo manifest

  ```bash
  cd ~/workspace/diego-release
  ./generate_combo_manifest warden ../cf-release ~/workspace/deployments/warden/director.yml > ~/workspace/deployments/warden/diego.yml
  ```
 
1. Target the deployment

  ```
  bosh deployment ~/workspace/deployments/warden/diego.yml
  ```

1. Create and upload the releases

  ```bash
  cd ~/workspace/diego-release
  bosh create release --force
  yes yes | bosh upload release
  
  cd ~/workspace/cf-release
  bosh create release --force
  yes yes | bosh upload release
  ```

1. Deploy!

  ```
  bosh deploy
  ```

1. Login to the locally deployed CF
  ```
  cf api api.10.244.0.34.xip.io
  cf login
  <admin/admin>
  ```

1. Create new CF Org & Space

  ```
  cf api api.10.244.0.34.xip.io
  cf auth admin admin
  cf create-org diego
  cf create-space diego
  cf target -o diego -s diego
  ```

1. Checkout cf-acceptance-tests (to get hello-world app)

  ```bash
  cd ~/go
  go get -u -v github.com/cloudfoundry/cf-acceptance-tests/...
  cd ~/go/src/github.com/cloudfoundry/cf-acceptance-tests/assets/hello-world
  ```

1. Push hello-world app to CF & Configure it to use Diego

  ```
  cf push hello
  cf set-env hello CF_DIEGO_BETA true
  cf push hello
  ```
