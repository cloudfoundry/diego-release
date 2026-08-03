# Generate github actions from template

ytt -f ./gh_template.yml -f [ytt-helpers.star](https://github.com/cloudfoundry/wg-app-platform-runtime-ci/blob/main/shared/helpers/ytt-helpers.star) -f [index.yml](https://github.com/cloudfoundry/wg-app-platform-runtime-ci/blob/main/diego-release/index.yml) > ./workflows/tests-workflow.yml

## Supported jobs
- Template tests
- Lint repo
- Basic Verifications
- Unit and Integration tests (without DB)
- Unit and Integration tests (MySQL 5.7)
- Unit and Integration tests (Postgres)
- Unit and Integration tests (MySQL 8.0)

### How to run

Workflow runs automatically on pull requests targeting the `develop` branch.
