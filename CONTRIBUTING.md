# Contributing to Diego

The Diego team uses GitHub and accepts contributions via [pull request](https://help.github.com/articles/using-pull-requests).

The `diego-release` repository is a [BOSH](https://github.com/cloudfoundry/bosh) release for Diego. The root of this repository doubles as a Golang [`GOPATH`](https://golang.org/doc/code.html#GOPATH). For more information about configuring your Golang environment and automatically setting your `GOPATH` to the release directory, see the [instructions below](#initial-setup).

All Diego components are submodules in diego-release and can be found in the [`src/code.cloudfoundry.org`](https://github.com/cloudfoundry/diego-release/tree/master/src/code.cloudfoundry.org) directory of this repository.

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
cannot identify their CLA and ask them to sign either an individual or corporate CLA. This happens automatically as a
comment on pull requests.

When the project receives a new CLA, it is recorded in the project records, the CLA is added to the database for the
automated system uses, then we manually make the Pull Request as having a CLA on-file.


----
## Initial Setup
This BOSH release doubles as a `$GOPATH`. It will automatically be set up for you if you have [direnv](http://direnv.net) installed.

**NOTE:** diego-release and its components assume you're running the latest version of go. The project may not compile or work as expected with older versions of go.

    # create parent directory of diego-release
    mkdir -p ~/workspace
    cd ~/workspace

    # clone garden-runc-release
    git clone https://github.com/cloudfoundry/garden-runc-release.git
    pushd garden-runc-release
    git checkout master && git submodule update --init --recursive
    popd

    # clone diego-release
    git clone https://github.com/cloudfoundry/diego-release.git
    pushd diego-release/

    # automate $GOPATH and $PATH setup
    direnv allow

    # switch to develop branch to make changes to diego-release,
    git checkout develop

    # initialize and sync submodules
    ./scripts/update
    popd

If you do not wish to use direnv, you can simply `source` the `.envrc` file in the root of the release repo.  You may manually need to update your `$GOPATH` and `$PATH` variables as you switch in and out of the directory.

Check out and install `pre-commit` and `pre-push` git hooks, by running `./scripts/install-git-hooks`. This will ensure to catch common drifts for packaging spec and package imports.

To be able to run unit tests, you'll also need to install the following binaries:

    # Install ginkgo
    go install github.com/onsi/ginkgo/ginkgo

    # Install nats-server
    wget https://github.com/nats-io/nats-server/releases/download/v2.1.2/nats-server-v2.1.2-linux-amd64.zip
    unzip -j nats-server-v2.1.2-linux-amd64.zip nats-server-v2.1.2-linux-amd64/nats-server
    rm nats-server-v2.1.2-linux-amd64.zip
    mv ./nats-server "$GOBIN/nats-server"

    # Install consul
    if uname -a | grep Darwin; then os=darwin; else os=linux; fi
    curl -L -o $TMPDIR/consul-0.7.1.zip "https://releases.hashicorp.com/consul/0.7.1/consul_0.7.1_${os}_amd64.zip"
    unzip $TMPDIR/consul-0.7.1.zip -d $GOPATH/bin
    rm $TMPDIR/consul-0.7.1.zip

To be able to run the integration test suite ("inigo"), you'll need to have a local [Concourse](http://concourse.ci) VM. Follow the instructions on the Concourse [README](https://github.com/concourse/concourse/blob/master/README.md) to set it up locally using [vagrant](https://www.vagrantup.com/). Download the fly CLI as instructed and move it somewhere visible to your `$PATH`.


## Code Conventions
### Metrics

Metrics added to any of the Diego components need to follow the naming and documentation conventions listed here, as otherwise the Diego unit tests will fail.

- Metrics must be defined as constants, preferably at the beginning of the file.
- The constant declaration for the metric needs to follow the format `ConstantMetricName = "metric name"`.
- All component level metrics passed to `diego-logging-client.IngressClient` must be documented in the [metrics documentation](docs/metrics.md).
- Application level metrics passed to `diego-logging-client.SendApp*` should not be documented.

### Running the SQL unit tests

As of Diego 1.0, SQL unit tests are the default unit tests for Diego. To run the SQL unit tests locally requires running MySQL and Postgres with the correct configuration. The recommended way to run databases is by using Docker.

#### Setup Mysql

1. Write out the MySQL config:

        echo -e "[mysqld]\nsql_mode=NO_ENGINE_SUBSTITUTION,STRICT_TRANS_TABLES\ndefault_authentication_plugin=mysql_native_password" > my.cnf

1. Run MySQL container. Note: Your `my.cnf` should be in the current directory:

        docker run \
            --rm \
            --detach \
            --name mysql \
            -p 3306:3306 \
            -e MYSQL_ROOT_PASSWORD=diego \
            -v my.cnf:/etc/mysql/conf.d/my.cnf \
            --tmpfs /var/lib/mysql:rw \
            mysql

1. Run the following SQL commands to create a diego user with the correct permissions:

        // manual password entry: diego
        mysql -uroot -p -h127.0.0.1
        CREATE USER 'diego'@'%' IDENTIFIED WITH mysql_native_password BY 'diego_password';
        GRANT ALL PRIVILEGES ON `diego\_%`.* TO 'diego'@'%';
        GRANT ALL PRIVILEGES ON `routingapi\_%`.* TO 'diego'@'%';

#### Setup Postgres

1. Create a self-signed certificate as described in the [PostgreSQL documentation](https://www.postgresql.org/docs/9.4/static/ssl-tcp.html#SSL-CERTIFICATE-CREATION).
   Save the certificate and key to a local directory of your choosing.

1. Set the owner to the postgres user:

        sudo chown 999:999 server.key server.crt

1. Run Postgres container. Note: Your `server.crt` and `server.key` should be in the current directory:

        docker run \
            --rm \
            --detach \
            --name pg \
            -p 5432:5432 \
            -e POSTGRES_PASSWORD=diego_pw \
            -e POSTGRES_DB=diego \
            -e POSTGRES_USER=diego \
            -v $PWD/server.crt:/var/lib/postgresql/server.crt \
            -v $PWD/server.key:/var/lib/postgresql/server.key \
            postgres \
                -c ssl=on \
                -c max_connections=300 \
                -c ssl_cert_file=/var/lib/postgresql/server.crt \
                -c ssl_key_file=/var/lib/postgresql/server.key

#### Run unit tests

To run all the SQL-backed tests, run the following command from the root of diego-release:

        ./scripts/run-unit-tests

This command will run all regular unit tests, as well as BBS and component integration tests where a backing store is required in MySQL-backed and Postgres-backed modes.

## <a name="deploy-bosh-lite"></a> Deploying Diego to BOSH-Lite

1. Install and start [BOSH-Lite](https://github.com/cloudfoundry/bosh-lite),
   following its
   [README](https://github.com/cloudfoundry/bosh-lite/blob/master/README.md).

1. [Download the latest Warden Trusty Go-Agent stemcell](https://bosh.io/stemcells) and upload it to BOSH-lite:

        bosh upload-stemcell (downloaded filename)

1. Check out cf-deployment (release-candidate branch) from git:

        cd ~/workspace
        git clone https://github.com/cloudfoundry/cf-deployment.git
        cd ~/workspace/cf-deployment
        git checkout release-candidate

1. Check out diego-release (develop branch) from git:

        cd ~/workspace
        git clone https://github.com/cloudfoundry/diego-release.git
        cd ~/workspace/diego-release
        git checkout develop
        ./scripts/update

1. Check out [instructions for deploying CF to local bosh-lite](https://github.com/cloudfoundry/cf-deployment/blob/master/deployment-guide.md#for-operators-deploying-cf-to-local-bosh-lite)

1. In order to use the latest diego-release, create and pass the following opsfile when deploying using bosh:
    ```
    ---
    - type: replace
      path: /releases/name=diego
      value:
        name: diego
        url: file://PATH_TO_HOME/workspace/diego-release
        version: create
    ```

1. Login to CF and enable Docker support:

        cf login -a api.bosh-lite.com -u admin -p admin --skip-ssl-validation &&
        cf enable-feature-flag diego_docker

Now you are configured to push an app to the BOSH-Lite deployment, or to run the
[CF Smoke Tests](https://github.com/cloudfoundry/cf-smoke-tests)
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

## Logging in Diego

Please follow logging conventions as outlined [here](https://github.com/cloudfoundry/diego-dev-notes/blob/master/notes/logging-guidance.md).


## Testing Diego

### Running Unit Tests
Once you've followed the steps [above](#initial-setup) to install ginkgo and the other binaries needed for testing, execute the following script to run all unit tests in diego-release.

    ./scripts/run-unit-tests

We recommend running the unit tests against both a local MySQL and a local PostgreSQL database as described [above](#running-the-sql-unit-tests).

If you want to run the entire unit test suite on concourse and have the `fly` CLI on your path, you can run

    ./scripts/run-unit-tests-concourse

from the root of diego-release. By default this script will attempt to run the unit tests on your local concourse installation, but you can change your concourse target by setting the `DIEGO_CI_TARGET` environment variable.

### Running Integration Tests

If your local concourse VM is up and running, you have the `fly` CLI visible on your `$PATH`, and you've cloned garden-runc-release (see [Initial Setup](#initial-setup) for details), you can run

    ./scripts/run-inigo

from the root of diego-release to run the integration tests. You can also run the integration tests against another concourse deployment by setting the `DIEGO_CI_TARGET` environment variable.

### <a name="smokes-and-cats"></a> Running Smoke Tests, and CATs

You can test that your diego-release deployment is working and integrating with cf by running the lightweight [cf-smoke-tests](https://github.com/cloudfoundry/cf-smoke-tests) or the more thorough [cf-acceptance-tests](https://github.com/cloudfoundry/cf-acceptance-tests). These test suites assume you have a BOSH environment to deploy cf and diego to. For local development, bosh-lite is an easy way to have a single-VM deployment. To deploy diego to bosh-lite, follow the instructions on [deploying diego to bosh-lite](README.md#deploy-bosh-lite).

The instructions below assume you're using bosh-lite and have generated the
manifests with the `scripts/generate-bosh-lite-manifests` script. This script
will also generate manifests for the errands that run these test suites. If you
did not run that script or are running tests in a different environment,
substitute the relevant manifest files in the `bosh deploy` commands below.

To run the cf-acceptance-tests against a Diego deployed to bosh-lite, run:

    ./scripts/run-cats-bosh-lite

To run cf-smoke-tests you can similarly deploy and run an errand to run the tests:

        # target the errand for smoke tests when running them
        bosh -n -d bosh-lite/deployments/cf.yml run errand smoke_tests

### Running DUSTs in a container

**Note**: The test suite mentioned below is experimental.

To run the Diego Upgrade Stability Tests (aka DUSTs), you will need an earlier version of the code checked out in a `diego-release-v0` directory. This directory should have the same parent directory as your `diego-release` repo:

    git clone https://github.com/cloudfoundry/diego-release diego-release-v0
    pushd diego-release-v0
    git checkout <VERSION_V0>
    ./scripts/update
    git clean -dff
    popd

We currently test our upgrades from v1.0.0 and v1.25.2 (the first version to add Locket).

Once you have a V0 version of Diego, run the following command in the newer version of Diego to create a docker container that is suitable for running inigo or vizzini:

    cd diego-release
    ./scripts/start-inigo-container

**Warning**: The script assumes that you follow the team's conventions:
1. Your older version of [diego-release](https://github.com/cloudfoundry/diego-release) is cloned into `~/workspace/diego-release-v0`.
1. Your newer (modified) version of [diego-release](https://github.com/cloudfoundry/diego-release) is cloned into `~/workspace/diego-release`.
1. [garden-runc-release](https://github.com/cloudfoundry/garden-runc-release) is cloned into `~/workspace/garden-runc-release`.
1. [routing-release](https://github.com/cloudfoundry/routing-release) is cloned into `~/workspace/routing-release`.

The script will start a shell inside the container and setup the container appropriately. Navigate to the directory of the collocated DUSTs test suite:

    cd /diego-release/src/code.cloudfoundry.org/diego-upgrade-stability-tests/

You will need to set the following environment variables:
 - `DIEGO_VERSION_V0`, the V0 version in the environment variables by running `export DIEGO_VERSION_V0="v1.0.0"` or `export DIEGO_VERSION_V0="v1.25.2"`. If
you don't set this environment variable, the DUSTs will fail.
- `GRACE_TARBALL_CHECKSUM` to the SHA1 checksum of the grace
tarball that can be found [here](https://github.com/cloudfoundry/diego-release/blob/9d995def9b692e6796e671eceb269d769db89997/jobs/vizzini/spec#L81-L83).
- `DEFAULT_ROOTFS` to the rootfs located in the
inigo container: `export DEFAULT_ROOTFS=/tmp/rootfs.tar`.

Now you can run the tests by running `ginkgo`.

Note that this suite can not currently be run using multiple ginkgo nodes due to a limitation around port configuration for the file server in Diego 1.0.0. Make sure not to include the `-p` or `-nodes` flags in your Ginkgo run.

The test suite will start all necessary Diego dependencies and related components, then run upgrade tests against various configurations of those components. This includes route availability and Diego API features.
