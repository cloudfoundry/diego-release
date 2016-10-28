## Deploying CF and Diego to BOSH-Lite

1. Install and start [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite),
   following its
   [README](https://github.com/cloudfoundry/bosh-lite/blob/master/README.md).
   For garden-runc to function properly in the Diego deployment,
   we recommend using version 9000.69.0 or later of the BOSH-Lite Vagrant box image.

1. Upload the latest version of the Warden BOSH-Lite stemcell directly to BOSH-Lite:

  ```
  bosh upload stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent
  ```

  Alternately, download the stemcell locally first and then upload it to BOSH-Lite:

  ```
  curl -L -o bosh-lite-stemcell-latest.tgz https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent
  bosh upload stemcell bosh-lite-stemcell-latest.tgz
  ```

  Please note that the consul_agent job does not set up DNS correctly on version 3126 of the Warden BOSH-Lite stemcell, so we do not recommend the use of that stemcell version.

1. Check out cf-release (release-candidate branch or tagged release) from git:

  ```bash
  cd ~/workspace
  git clone https://github.com/cloudfoundry/cf-release.git
  cd ~/workspace/cf-release
  git checkout release-candidate # do not push to release-candidate
  ./scripts/update
  ```

1. Check out diego-release (master branch or tagged release) from git:

  ```bash
  cd ~/workspace
  git clone https://github.com/cloudfoundry/diego-release.git
  cd ~/workspace/diego-release
  git checkout master # do not push to master
  ./scripts/update
  ```

1. Install `spiff` according to its [README](https://github.com/cloudfoundry-incubator/spiff).
   `spiff` is a tool for generating BOSH manifests that is required in some of the scripts used below.

1. Generate the CF manifest:

  ```bash
  cd ~/workspace/cf-release
  ./scripts/generate-bosh-lite-dev-manifest
  ```

   **Or if you are running Windows cells** along side this deployment, instead generate the CF manifest as follows:

  ```bash
  cd ~/workspace/cf-release
  ./scripts/generate-bosh-lite-dev-manifest \
    ~/workspace/diego-release/manifest-generation/stubs-for-cf-release/enable_diego_windows_in_cc.yml
  ```

1. Generate the Diego manifests:

  ```bash
  cd ~/workspace/diego-release
  ./scripts/generate-bosh-lite-manifests
  ```

  1. If using MySQL run the following to enable it on Diego:

     ```bash
     cd ~/workspace/diego-release
     USE_SQL='mysql' ./scripts/generate-bosh-lite-manifests
     ```

  1. If using Postgres run the following to enable it on Diego:

     ```bash
     cd ~/workspace/diego-release
     USE_SQL='postgres' ./scripts/generate-bosh-lite-manifests
     ```

1. Create, upload, and deploy the CF release:

  ```bash
  cd ~/workspace/cf-release
  bosh deployment bosh-lite/deployments/cf.yml
  bosh -n create release --force &&
  bosh -n upload release &&
  bosh -n deploy
  ```

1. If configuring Diego to use MySQL, upload and deploy the latest cf-mysql-release:

  ```bash
  cd ~/workspace/diego-release
  bosh upload release https://bosh.io/d/github.com/cloudfoundry/cf-mysql-release
  ./scripts/generate-mysql-bosh-lite-manifest
  bosh deployment bosh-lite/deployments/cf-mysql.yml
  bosh -n deploy
  ```

  1. Accessing the MySQL remotely:

    You can access the mysql database used as deployed above with the following command:

    ```bash
    mysql -h 10.244.7.2 -udiego -pdiego diego
    ```

    Then commands such as `SELECT * FROM desired_lrps` can be run to show all the desired lrps in the system.

1. Upload the latest garden-runc-release:

  ```bash
  bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-runc-release
  ```

  If you wish to upload a specific version of garden-runc-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry-incubator/garden-runc-release).

1. Upload the latest etcd-release:

  ```bash
  bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release
  ```

  If you wish to upload a specific version of etcd-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry-incubator/etcd-release).

1. Upload the latest cflinuxfs2-rootfs-release:

  ```bash
  bosh upload release https://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-rootfs-release
  ```

  If you wish to upload a specific version of cflinuxfs2-rootfs-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry/cflinuxfs2-rootfs-release).

1. Create, upload, and deploy the Diego release:

  ```bash
  cd ~/workspace/diego-release
  bosh deployment bosh-lite/deployments/diego.yml
  bosh -n create release --force &&
  bosh -n upload release &&
  bosh -n deploy
  ```

  If deploying using garden-runc after already deploying using garden-linux the cells must be recreated.  Pass the --recreate flag to the deploy command.

  ```bash
  cd ~/workspace/diego-release
  bosh deployment bosh-lite/deployments/diego.yml
  bosh -n create release --force &&
  bosh -n upload release &&
  bosh -n deploy --recreate
  ```

1. Login to CF and enable Docker support:

  ```bash
  cf login -a api.bosh-lite.com -u admin -p admin --skip-ssl-validation &&
  cf enable-feature-flag diego_docker
  ```

Now you are configured to push an app to the BOSH-Lite deployment, or to run the
[Smoke Tests](https://github.com/cloudfoundry/cf-smoke-tests)
or the
[CF Acceptance Tests](https://github.com/cloudfoundry/cf-acceptance-tests).

> If you wish to run all of the diego jobs on a single VM, you can replace the
> `manifest-generation/bosh-lite-stubs/instance-count-overrides.yml` stub with
> the `manifest-generation/bosh-lite-stubs/colocated-instance-count-overrides.yml`
> stub.
