# Contributing to Diego

The Diego team uses GitHub and accepts contributions via [pull request](https://help.github.com/articles/using-pull-requests).

The `diego-release` repository is a [BOSH](https://github.com/cloudfoundry/bosh) release for Diego.

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
**NOTE:** diego-release and its components assume you're running the latest version of go. The project may not compile or work as expected with older versions of go.

    # create parent directory of diego-release
    mkdir -p ~/workspace
    cd ~/workspace

    # clone ci
    git clone https://github.com/cloudfoundry/wg-app-platform-runtime-ci.git

    # clone diego-release
    git clone https://github.com/cloudfoundry/diego-release.git
    pushd diego-release/
    git submodule update --init --recursive

    # switch to develop branch to make changes to diego-release,
    git checkout develop

    popd


To be able to run the integration test suite ("inigo"), you'll need to have a local [Concourse](http://concourse.ci) VM. Follow the instructions on the Concourse [README](https://github.com/concourse/concourse/blob/master/README.md) to set it up locally using [vagrant](https://www.vagrantup.com/). Download the fly CLI as instructed and move it somewhere visible to your `$PATH`.


## Code Conventions
### Metrics

Metrics added to any of the Diego components need to follow the naming and documentation conventions listed here, as otherwise the Diego unit tests will fail.

- Metrics must be defined as constants, preferably at the beginning of the file.
- The constant declaration for the metric needs to follow the format `ConstantMetricName = "metric name"`.
- All component level metrics passed to `diego-logging-client.IngressClient` must be documented in the [metrics documentation](docs/metrics.md).
- Application level metrics passed to `diego-logging-client.SendApp*` should not be documented.

----

#### <a name="running-unit-and-integration-tests"></a> Running Unit and Integration Tests

##### With Docker

Running tests for this release requires a `DB` flavor. The following scripts with default to `mysql` DB. Set `DB` environment variable for alternate DBs e.g. <mysql-8.0(or mysql),mysql-5.7,postgres>

- `./scripts/create-docker-container.bash`: This will create a docker container with appropriate mounts.
- `./scripts/test-in-docker-locally.bash`: Create docker container and run all tests and setup in a single script.
  - `./scripts/test-in-docker-locally.bash <package> <sub-package>`: For running tests under a specific package and/or sub-package: e.g. `./scripts/test-in-docker-locally.bash executor`

When inside docker container: 
- `/repo/scripts/docker/build-binaries.bash`: This will build binaries required for running tests e.g. nats-server and rtr
- `/repo/scripts/docker/test.bash`: This will run all tests in this release
- `/repo/scripts/docker/test.bash executor`: This will only run `executor` tests
- `/repo/scripts/docker/test.bash executor initializer`: This will only run `initializer` sub-package tests for `executor` package
- `/repo/scripts/docker/tests-templates.bash`: This will run all of tests for bosh tempalates
- `/repo/scripts/docker/lint.bash`: This will run all of linting defined for this repo.

## Logging in Diego

Please follow logging conventions as outlined [here](https://github.com/cloudfoundry/diego-dev-notes/blob/master/notes/logging-guidance.md).
