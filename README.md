# Cloud Foundry Diego (BOSH release) [![slack.cloudfoundry.org](https://slack.cloudfoundry.org/badge.svg)](https://slack.cloudfoundry.org)

----
This repo is a [BOSH](https://github.com/cloudfoundry/bosh) release for
deploying Diego and associated tasks for testing a Diego deployment.
Diego is the new container runtime system for Cloud Foundry, replacing the DEAs and Health Manager.

This release relies on a separate deployment to provide
[Consul](https://github.com/hashicorp/consul),
[NATS](https://github.com/apcera/gnatsd), and
[Loggregator](https://github.com/cloudfoundry/loggregator). In practice, these typically
come from [cf-release](https://github.com/cloudfoundry/cf-release).

The [Diego Design Notes](https://github.com/cloudfoundry/diego-design-notes) present an overview of Diego, and links to the various Diego components.

## Table of Contents

1. [Diego Operator Resources](#diego-operator-resources)
  1. [Deployment Examples: BOSH-Lite and AWS](#deployment-examples)
  1. [Deployment Requirements and Constraints](#deployment-requirements-constraints)
  1. [Deploying Diego-Backed Cloud Foundry](#deploying-diego-backed-cloud-foundry)
  1. [Security Configuration](#security-configuration)
  1. [Data Store Configuration](#data-store-configuration)
  1. [Component Coordination](#component-coordination)
  1. [Monitoring and Inspection](#monitoring-inspection)
1. [CF App Developer Resources](#cf-app-developer-resources)
1. [Diego Contributor Resources](#diego-contributor-resources)

---

## <a name="diego-operator-resources"></a>Diego Operator Resources

### <a name="deployment-examples"></a>Deployment Examples: BOSH-Lite and AWS

- [Deploying CF and Diego to BOSH-Lite](examples/bosh-lite) provides detailed instructions for deploying a Diego-backed CF to a BOSH-Lite instance.
- [Deploying CF and Diego to AWS](examples/aws) provides detailed instructions to deploy BOSH, CF, and Diego to a new CloudFormation stack. Alternately, follow the [instructions in cf-release](https://github.com/cloudfoundry/cf-release/tree/master/example_manifests) to deploy Diego alongside a minimal CF deployment.


### <a name="deployment-requirements-constraints"></a>Deployment Requirements and Constraints

- [Required BOSH Versions](docs/required-bosh-versions.md) details the minimum versions of the BOSH director, stemcell, and dependency releases required to deploy Diego correctly.
- [Deployment Constraints](docs/deployment-constraints.md) describes the dependencies that must be deployed before deploying the Diego cluster and restrictions on Diego instance update order and rates to ensure correct cluster operation.


### <a name="deploying-diego-backed-cloud-foundry"></a>Deploying Diego-Backed Cloud Foundry

- Diego is typically deployed alongside a Cloud Foundry deployment to serve as its new container runtime. See "[Deploying Diego Alongside an Existing CF Deployment](docs/deploy-alongside-existing-cf.md)" for general instructions and guidelines to deploy Diego this way.
- [Diego Manifest Generation](docs/manifest-generation.md) describes the manifest-generation scripts in this repository.
- [Release Compatibility](docs/release-compatibility.md) illustrates how to select versions of CF and other BOSH releases to deploy alongside Diego.
- [Managing the Migration](https://github.com/cloudfoundry/diego-design-notes/blob/master/migrating-to-diego.md#managing-the-migration) describes how operators can manage a transition from the DEAs to Diego.


### <a name="security-configuration"></a>Security Configuration

- [TLS Configuration](docs/tls-configuration.md) describes how to generate TLS certificates for secure communication with Consul, the Diego BBS, and the Diego cell reps.
- [Upgrading the cell rep API to mutual TLS](docs/upgrading-secure-cell-rep-api.md) explains how to transition an existing Diego deployment to use mutual TLS for communication to the cell rep API without incurring downtime.
- [Upgrading the auctioneer API to mutual TLS](docs/upgrading-secure-auctioneer-api.md) explains how to transition an existing Diego deployment to use mutual TLS for communication from the BBS to the auctioneer API without incurring downtime.
- (**Experimental**) [Instance Identity](docs/instance-identity.md) explains how to enable instance identity.


### <a name="data-store-configuration"></a>Data Store Configuration

- [Supported Data Stores for Diego](docs/data-stores.md) describes how to configure Diego to use either SQL for its data store and how to arrange automatic migration of data from etcd to MySQL or Postgres for old deployment that are using etcd.
- [Data Store Encryption](docs/data-store-encryption.md) explains how to manage the ring of encryption keys that Diego uses to secure data at rest.
- [Performance Tuning](docs/performance-tuning.md) describes potential performance improvement recommendations.


### <a name="component-coordination"></a>Component Coordination

- (**Experimental**) [Migrating from Consul to SQL Locks](docs/migrating-from-consul-to-sql-locks.md) explains how to migrate the BBS and auctioneer from coordinating around a lock in Consul to coordinating around one stored in the Diego relational database.


### <a name="monitoring-inspection"></a>Monitoring and Inspection

- [Diego Metrics](docs/metrics.md) lists the various metrics that Diego emits through the Loggregator system.
- [`cfdot` Setup](docs/cfdot-setup.md) shows how to set up the `cfdot` CF Diego Operator Tool CLI for use in inspecting and interacting with a Diego deployment.


## <a name="cf-app-developer-resources"></a>CF App Developer Resources

- [Migrating to Diego](https://github.com/cloudfoundry/diego-design-notes/blob/master/migrating-to-diego.md) describes how developers can switch from the DEAs to Diego and details various operational differences between the DEAs and Diego.
- The [Docker Support Notes](https://github.com/cloudfoundry/diego-design-notes/blob/master/docker-support.md) describe how Diego runs Docker-image-based apps in Cloud Foundry.


## <a name="diego-contributor-resources"></a>Diego Contributor Resources

- The [Contribution Guidelines](CONTRIBUTING.md) describes the developer workflow for making changes to Diego.
- The [CF Runtime Diego Pivotal Tracker project](https://www.pivotaltracker.com/n/projects/1003146) shows active areas of work for the Diego team in the backlog section.
- The [Diego Dev Notes](https://github.com/cloudfoundry/diego-dev-notes) provide a detailed explanation of how the Diego components and internal state machine interact, as well as information on development workstation setup.
- The [BBS Benchmarks](docs/bbs-benchmarks.md) provides information about results from the BBS benchmark tests that run in the Diego team's continuous integration testing pipeline.
