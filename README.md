# Cloud Foundry Diego (BOSH release) [![slack.cloudfoundry.org](https://slack.cloudfoundry.org/badge.svg)](https://slack.cloudfoundry.org)

----
This repository is a [BOSH](https://github.com/cloudfoundry/bosh) release for
deploying Diego and associated tasks for testing a Diego deployment.
Diego is the new container runtime system for Cloud Foundry, replacing the DEAs and Health Manager.

This release depends on external services such as a relational database (either [MySQL](https://github.com/cloudfoundry/cf-mysql-release) or [Postgres](https://github.com/cloudfoundry/postgres-release)) for data storage and [Consul](https://github.com/hashicorp/consul) or [BOSH DNS](https://github.com/cloudfoundry/bosh-dns-release) for inter-component service discovery. It also integrates with [NATS](https://github.com/nats-io/gnatsd) to register routes to applications and [Loggregator](https://github.com/cloudfoundry/loggregator) to emit application logs and Diego component metrics. In practice, these dependencies typically come from [cf-deployment](https://github.com/cloudfoundry/cf-deployment).

The [Diego Design Notes](https://github.com/cloudfoundry/diego-design-notes) present an overview of Diego, and links to the various Diego components.

## Table of Contents

1. [Diego Operator Resources](#diego-operator-resources)
    1. [Deploying Diego-Backed Cloud Foundry](#deploying-diego-backed-cloud-foundry)
    1. [Deployment Examples](#deployment-examples)
    1. [Deployment Requirements and Constraints](#deployment-requirements-constraints)
    1. [Security Configuration](#security-configuration)
    1. [Data Store Configuration](#data-store-configuration)
    1. [Component Coordination](#component-coordination)
    1. [Monitoring and Inspection](#monitoring-inspection)
1. [CF App Developer Resources](#cf-app-developer-resources)
1. [Diego Contributor Resources](#diego-contributor-resources)

---

## <a name="diego-operator-resources"></a>Diego Operator Resources

### <a name="deploying-diego-backed-cloud-foundry"></a>Deploying Diego-Backed Cloud Foundry

Diego is typically deployed as part of a Cloud Foundry Application Runtime deployment to serve as its container runtime. The [cf-deployment](https://github.com/cloudfoundry/cf-deployment) repository contains the latest recommended way to use BOSH to deploy a Cloud Foundry cluster to infrastructure platforms such as AWS, GCP, and Azure.

- [Release Compatibility](docs/release-compatibility.md) illustrates how to select versions of CF and other BOSH releases to deploy alongside Diego.

### <a name="deployment-examples"></a>Deployment Examples

#### Deploying to BOSH-Lite

- Create a BOSH-Lite VM using either the [v2 BOSH CLI](https://bosh.io/docs/bosh-lite.html) or [bosh-bootloader](https://github.com/cloudfoundry/cf-deployment/tree/master/iaas-support/bosh-lite). Note that to create a BOSH-Lite VM in your local VirtualBox, you must use the BOSH CLI.
- Follow the instructions in [CF-Deployment](https://github.com/cloudfoundry/cf-deployment/tree/master/iaas-support/bosh-lite#5-upload-the-cloud-config) to deploy CF to the BOSH-Lite VM.


### <a name="deployment-requirements-constraints"></a>Deployment Requirements and Constraints

- [Required Dependency Versions](docs/required-dependency-versions.md) details the minimum versions of the BOSH director, stemcell, and dependency releases required to deploy Diego correctly.
- [Deployment Constraints](docs/deployment-constraints.md) describes the dependencies that must be deployed before deploying the Diego cluster and restrictions on Diego instance update order and rates to ensure correct cluster operation.
- [Deprecations](docs/deprecations.md) lists deprecated BOSH job properties, component metrics, and endpoints and fields for Diego component APIs.


### <a name="security-configuration"></a>Security Configuration

- [TLS Configuration](docs/tls-configuration.md) describes how to generate TLS certificates for secure communication amongst the Diego and greater CF components.
- [Instance Identity](docs/instance-identity.md) explains how to enable the Diego cell reps to provide per-instance identity credentials.


### <a name="data-store-configuration"></a>Data Store Configuration

- [Supported Data Stores for Diego](docs/data-stores.md) describes how to configure Diego to use SQL for its data store.
- [Data Store Encryption](docs/data-store-encryption.md) explains how to manage the ring of encryption keys that Diego uses to secure data at rest.
- [Performance Tuning](docs/performance-tuning.md) describes potential performance improvement recommendations.


### <a name="component-coordination"></a>Component Coordination

- [Migrating from Consul to SQL Locks](docs/migrating-from-consul-to-sql-locks.md) explains how to migrate the BBS and auctioneer from coordinating around a lock in Consul to coordinating around one stored in the Diego relational database.


### <a name="monitoring-inspection"></a>Monitoring and Inspection

- [Diego Metrics](docs/metrics.md) lists the various metrics that Diego emits through the Loggregator system.
- [`cfdot` Setup](docs/cfdot-setup.md) shows how to set up the `cfdot` CF Diego Operator Tool CLI for use in inspecting and interacting with a Diego deployment.
- [Logging format](docs/logging-format.md) shows how to configure the logging format for Diego components.

## <a name="cf-app-developer-resources"></a>CF App Developer Resources

- [Migrating to Diego](https://github.com/cloudfoundry/diego-design-notes/blob/master/migrating-to-diego.md) describes how developers can switch from the DEAs to Diego and details various operational differences between the DEAs and Diego.
- The [Docker Support Notes](https://github.com/cloudfoundry/diego-design-notes/blob/master/docker-support.md) describe how Diego runs Docker-image-based apps in Cloud Foundry.


## <a name="diego-contributor-resources"></a>Diego Contributor Resources

- The [Contribution Guidelines](CONTRIBUTING.md) describes the developer workflow for making changes to Diego.
- The [CF Runtime Diego Pivotal Tracker project](https://www.pivotaltracker.com/n/projects/1003146) shows active areas of work for the Diego team in the backlog section.
- The [Diego Dev Notes](https://github.com/cloudfoundry/diego-dev-notes) provide a detailed explanation of how the Diego components and internal state machine interact, as well as information on development workstation setup.
- The [BBS Benchmarks](docs/bbs-benchmarks.md) provides information about results from the BBS benchmark tests that run in the Diego team's continuous integration testing pipeline.
