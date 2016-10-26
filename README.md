# BOSH release for Cloud Foundry Diego [![slack.cloudfoundry.org](https://slack.cloudfoundry.org/badge.svg)](https://slack.cloudfoundry.org)

----
This repo is a [BOSH](https://github.com/cloudfoundry/bosh) release for
deploying Diego and associated tasks for testing a Diego deployment.
Diego is the new container runtime system for Cloud Foundry, replacing the DEAs and Health Manager.

This release relies on a separate deployment to provide
[Consul](https://github.com/hashicorp/consul),
[NATS](https://github.com/apcera/gnatsd), and
[Loggregator](https://github.com/cloudfoundry/loggregator). In practice, these typically
come from [cf-release](https://github.com/cloudfoundry/cf-release).

## Table of Contents

1. [Deployment Examples](#deployment-examples)
  1. [BOSH-Lite](#bosh-lite)
  1. [AWS](#aws)
1. [Additional Diego Resources](#additional-diego-resources)
1. [Required BOSH Versions](#required-bosh-versions)
1. [Deploying Alongside an Existing CF Deployment](#deploy-alongside-cf)
1. [Release Compatibility](#release-compatibility)
1. [Manifest Generation](#manifest-generation)
1. [Deployment Constraints](#deployment-constraints)
1. [Pushing a CF Application to the Diego Runtime](#pushing-to-diego)
1. [BBS Benchmarks](#bbs-benchmarks)

---

## <a name="deployment-examples"></a>Deployment Examples

### <a name="bosh-lite"></a>BOSH-Lite

To deploy CF and Diego to BOSH-Lite, follow the instructions at [examples/bosh-lite](examples/bosh-lite).


### <a name="aws"></a>AWS

In order to deploy Diego to AWS, follow the instructions in [examples/aws](examples/aws/README.md) to deploy BOSH, CF, and Diego to a new CloudFormation stack, or follow the instructions in [examples/minimal-aws](examples/minimal-aws/README.md) to deploy Diego alongside a [minimal CF deployment](https://github.com/cloudfoundry/cf-release/tree/master/example_manifests).


## <a name="additional-diego-resources"></a>Additional Diego Resources

- The [Contribution Guidelines](CONTRIBUTING.md) describes the developer workflow for making changes to Diego.
- The [Diego Design Notes](https://github.com/cloudfoundry/diego-design-notes) present an overview of Diego, and links to the various Diego components.
- The [Migration Guide](https://github.com/cloudfoundry/diego-design-notes/blob/master/migrating-to-diego.md) describes how developers and operators can manage a transition from the DEAs to Diego.
- The [Docker Support Notes](https://github.com/cloudfoundry/diego-design-notes/blob/master/docker-support.md) describe how Diego runs Docker-image-based apps in Cloud Foundry.
- [Supported Data Stores for Diego](docs/data-stores.md)
- [Data Store Encryption](docs/data-store-encryption.md)
- [TLS Configuration](docs/tls-configuration.md)
- [Upgrade the cell rep API to mutual TLS](docs/upgrade-to-secure-cell-rep-api.md) without downtime.
- [Diego's Pivotal Tracker project](https://www.pivotaltracker.com/n/projects/1003146) shows what we're working on these days.
- [Diego Metrics](docs/metrics.md) lists the various metrics that Diego emits through the Loggregator system.


## <a name="required-bosh-versions"></a>Required BOSH Versions

See [Required BOSH Versions](docs/required-bosh-versions.md) for information about the minimum BOSH director and stemcell versions required to deploy Diego correctly.

## <a name="deploy-alongside-cf"></a>Deploying Alongside an Existing CF Deployment

Diego is typically deployed alongside a CF deployment to serve as its new container runtime. See "[Deploying Diego Alongside an Existing CF Deployment](docs/deploy-alongside-existing-cf.md)" for general instructions and guidelines to deploy Diego this way.

## <a name="release-compatibility"></a>Release Compatibility

See [Release Compatibility](docs/release-compatibility.md) for information about selecting versions of CF and other BOSH releases to deploy alongside Diego.

## <a name="manifest-generation"></a>Manifest Generation

The Diego manifest generation documentation can be found in [docs/manifest-generation.md](docs/manifest-generation.md).

## <a name="deployment-constraints"></a>Deployment Constraints

See [Deployment Constraints](docs/deployment-constraints.md) for information about dependencies that must be deployed before deploying the Diego cluster and about restrictions on job update order and rates.

## <a name="pushing-to-diego"></a>Pushing a CF Application to the Diego Runtime

See [Pushing a CF Application to the Diego Runtime](docs/push-cf-app-to-diego.md) for instructions on pushing a CF app specifically to the Diego runtime.

## <a name="bbs-benchmarks"></a>BBS Benchmarks

See [BBS Benchmarks](docs/bbs-benchmarks.md) for more information about results from the BBS benchmark tests that run in the Diego team's continuous integration testing pipeline.
