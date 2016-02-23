

# Contributing to Diego

The Diego team uses GitHub and accepts contributions via [pull request](https://help.github.com/articles/using-pull-requests).

The `diego-release` repository is a [BOSH](https://github.com/cloudfoundry/bosh) release for Diego. The root of this repository doubles as a Golang [`GOPATH`](https://golang.org/doc/code.html#GOPATH). For more information about configuring your Golang environment and automatically setting your `GOPATH` to the release directory, see the [instructions below](#initial-setup).

All Diego components are submodules in diego-release and can be found in the [`src/github.com/cloudfoundry-incubator`](https://github.com/cloudfoundry-incubator/diego-release/tree/master/src/github.com/cloudfoundry-incubator) and [`src/github.com/cloudfoundry`](https://github.com/cloudfoundry-incubator/diego-release/tree/master/src/github.com/cloudfoundry) directories of this repository.

If you wish to make a change to an individual Diego component, submit a pull request to the master branches of its repository. Once accepted, those changes should make their way into `diego-release`.

If you wish to make a change to **diego-release** directly, please base your pull request on the **develop** branch, and not the master branch. The master branch of diego-release is reserved for the latest final BOSH release of Diego, and the only updates to that branch should be through our automated release-creation process.

To verify your changes before submitting a pull request, run unit tests, the inigo test suite, and the CF Acceptance Tests (CATs). See the [testing](#testing-diego) section for more detail.

---

## Contributor License Agreement

Follow these steps to make a contribution to any of our open source repositories:

1. Ensure that you have completed our CLA Agreement for [individuals](https://www.cloudfoundry.org/wp-content/uploads/2015/07/CFF_Individual_CLA.pdf) or [corporations](https://www.cloudfoundry.org/wp-content/uploads/2015/07/CFF_Corporate_CLA.pdf).

1. Set your name and email (these should match the information on your submitted CLA)
  ```
  git config --global user.name "Firstname Lastname"
  git config --global user.email "your_email@example.com"
  ```

1. All contributions must be sent using GitHub pull requests as they create a nice audit trail and structured approach.

The originating github user has to either have a github id on-file with the list of approved users that have signed
the CLA or they can be a public "member" of a GitHub organization for a group that has signed the corporate CLA.
This enables the corporations to manage their users themselves instead of having to tell us when someone joins/leaves an organization. By removing a user from an organization's GitHub account, their new contributions are no longer approved because they are no longer covered under a CLA.

If a contribution is deemed to be covered by an existing CLA, then it is analyzed for engineering quality and product
fit before merging it.

If a contribution is not covered by the CLA, then the automated CLA system notifies the submitter politely that we
cannot identify their CLA and ask them to sign either an individual or corporate CLA. This happens automatially as a
comment on pull requests.

When the project receives a new CLA, it is recorded in the project records, the CLA is added to the database for the
automated system uses, then we manually make the Pull Request as having a CLA on-file.


----
##Initial Setup
This BOSH release doubles as a `$GOPATH`. It will automatically be set up for you if you have [direnv](http://direnv.net) installed.

**NOTE:** diego-release and its components assume you're running go **1.4.3**. The project may not compile or work as expected with other versions of go.

    # create parent directory of cf-release and diego-release
    mkdir -p ~/workspace
    cd ~/workspace

    # clone cf-release
    git clone https://github.com/cloudfoundry/cf-release.git
    pushd cf-release/

    # if you're making changes to diego-release,
    git checkout release-candidate

    ## or, if you're making changes to cf-release,
    # git checkout develop

    ./scripts/update
    popd

    # clone garden-linux-release
    git clone https://github.com/cloudfoundry-incubator/garden-linux-release.git
    pushd garden-linux-release
    git checkout master && git submodule update --init --recursive
    popd

    # clone diego-release
    git clone https://github.com/cloudfoundry-incubator/diego-release.git
    pushd diego-release/

    # automate $GOPATH and $PATH setup
    direnv allow

    # switch to develop branch to make changes to diego-release,
    git checkout develop

    ## or, if you're only making changes to cf-release,
    # git checkout master

    # initialize and sync submodules
    ./scripts/update
    popd

If you do not wish to use direnv, you can simply `source` the `.envrc` file in the root of the release repo.  You may manually need to update your `$GOPATH` and `$PATH` variables as you switch in and out of the directory.

To be able to run unit tests, you'll also need to install the following binaries:

    # Install ginkgo
    go install github.com/onsi/ginkgo/ginkgo

    # Install gnatsd
    go install github.com/apcera/gnatsd

    # Install etcd
    go install github.com/coreos/etcd

    # Install consul
    if uname -a | grep Darwin; then os=darwin; else os=linux; fi
    curl -L -o $TMPDIR/consul-0.5.2.zip "https://releases.hashicorp.com/consul/0.5.2/consul_0.5.2_${os}_amd64.zip"
    unzip $TMPDIR/consul-0.5.2.zip -d $GOPATH/bin
    rm $TMPDIR/consul-0.5.2.zip

To be able to run the integration test suite ("inigo"), you'll need to have a local [Concourse](http://concourse.ci) VM. Follow the instructions on the Concourse [README](https://github.com/concourse/concourse/blob/master/README.md) to set it up locally using [vagrant](https://www.vagrantup.com/). Download the fly CLI as instructed and move it somewhere visible to your `$PATH`.

## <a name="deploy-bosh-lite"></a> Deploying Diego to BOSH-Lite

1. Install and start [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite),
   following its
   [README](https://github.com/cloudfoundry/bosh-lite/blob/master/README.md).

1. Download the latest Warden Trusty Go-Agent stemcell and upload it to BOSH-lite:

        bosh public stemcells
        bosh download public stemcell (name)
        bosh upload stemcell (downloaded filename)

1. Check out cf-release (runtime-passed branch) from git:

        cd ~/workspace
        git clone https://github.com/cloudfoundry/cf-release.git
        cd ~/workspace/cf-release
        git checkout runtime-passed
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
          ~/workspace/diego-release/manifest-generation/stubs-for-cf-release/enable_diego_windows_in_cc.yml

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

----
## Developer Workflow

When working on individual components of Diego, work out of the submodules under `src/`.

Run the individual component unit tests as you work on them using [ginkgo](https://github.com/onsi/ginkgo). To see if *everything* still works, run `./scripts/run-unit-tests` in the root of the release.

When you're ready to commit, run:

    ./scripts/prepare-to-diego <story-id> <another-story-id>...

This will synchronize submodules, update the BOSH package specs, run all unit tests, all integration tests, and make a commit, bringing up a commit edit dialogue.  The story IDs correspond to stories in our [Pivotal Tracker backlog](https://www.pivotaltracker.com/n/projects/1003146).  You should simultaneously also build the release and deploy it to a local [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite) environment, and run the acceptance tests.  See [Running Smoke Tests & CATs](#smokes-and-cats).

If you're introducing a new component (e.g. a new job/errand) or changing the main path for an existing component, make sure to update `./scripts/sync-package-specs` and `./scripts/sync-submodule-config`.

## Testing Diego

### Running Unit Tests
Once you've followed the steps [above](#initial-setup) to install ginkgo and the other binaries needed for testing, execute the following script to run all unit tests in diego-release.

    ./scripts/run-unit-tests


### Running Integration Tests

If your local concourse VM is up and running, you have the `fly` CLI visible on your `$PATH`, and you've cloned garden-linux-release (see [Initial Setup](#initial-setup) for details), you can run

    ./scripts/run-inigo
from the root of diego-release to run the integration tests.

###<a name="smokes-and-cats"></a> Running Smoke Tests, and CATs

You can test that your diego-release deployment is working and integrating with cf-release by running the lightweight [diego-smoke-tests](https://github.com/cloudfoundry-incubator/diego-smoke-tests) or the more thorough [cf-acceptance-tests](https://github.com/cloudfoundry/cf-acceptance-tests). These test suites assume you have a BOSH environment to deploy cf and diego to. For local development, bosh-lite is an easy way to have a single-VM deployment. To deploy diego to bosh-lite, follow the instructions on [deploying diego to bosh-lite](README.md#deploy-bosh-lite).

The instructions below assume you're using bosh-lite and have generated the
manifests with the `scripts/generate-bosh-lite-manifests` script. This script
will also generate manifests for the errands that run these test suites. If you
did not run that script or are running tests in a different environment,
substitute the relevant manifest files in the `bosh deploy` commands below.

To run the cf-acceptance-tests against a Diego deployed to bosh-lite, run:

    ./scripts/run-cats-bosh-lite

To run diego-smoke-tests you can similarly deploy and run an errand to run the tests, but you must make sure that the proper smoke org already exists:

        # create the org for bosh lite
        cf login -a api.bosh-lite.com -u admin -p admin --skip-ssl-validation
        cf create-org smoke-tests

        # deploy the errand for smoke tests
        bosh -n -d bosh-lite/deployments/diego-smoke-tests.yml deploy
        bosh -d bosh-lite/deployments/diego-smoke-tests.yml run errand diego_smoke_tests

### Running Benchmark Tests
Running the benchmark tests isn't usually needed for most changes. However, for  changes to the BBS or the protobuf models, it may be helpful to run these tests to understand the performance impact.

**WARNING**: Benchmark tests drop the database.

1. Deploy diego-release to an environment (use instance-count-overrides to turn
   off all components except the database for a cleaner test)

1. Depending on whether you're deploying to AWS or bosh-lite, copy either
   `manifest-generation/benchmark-errand-stubs/default_aws_benchmark_properties.yml` or
   `manifest-generation/benchmark-errand-stubs/default_bosh_lite_benchmark_properties.yml`
   to your local deployments or stubs folder and fill it in.

1. Generate a benchmark deployment manifest using:

        ./scripts/generate-benchmarks-manifest \
          /path/to/diego.yml \
          /path/to/benchmark-properties.yml \
          > benchmark.yml

1. Deploy and run the tests using:

        bosh -d benchmark.yml -n deploy && bosh -d benchmark.yml -n run errand benchmark-bbs

