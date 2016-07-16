# Cloud Foundry Diego [BOSH release] [![slack.cloudfoundry.org](https://slack.cloudfoundry.org/badge.svg)](https://slack.cloudfoundry.org)

----
This repo is a [BOSH](https://github.com/cloudfoundry/bosh) release for
deploying Diego and associated tasks for testing a Diego deployment.  Diego
builds out the new runtime architecture for Cloud Foundry, replacing the DEAs
and Health Manager.

This release relies on a separate deployment to provide
[Consul](https://github.com/hashicorp/consul),
[NATS](https://github.com/apcera/gnatsd) and
[Loggregator](https://github.com/cloudfoundry/loggregator). In practice these
come from [cf-release](https://github.com/cloudfoundry/cf-release).

## Table of Contents

1. [Deployment Examples](#deployment-examples)
  1. [BOSH-Lite](#bosh-lite)
  1. [AWS](#aws)
1. [Additional Diego Resources](#additional-diego-resources)
1. [BOSH Dependencies](#bosh-dependencies)
1. [Deploying Alongside an Existing CF Deployment](#deploy-alongside-cf)
1. [Release Compatibility](#release-compatibility)
1. [Manifest Generation](#manifest-generation)
1. [Deployment Constraints](#deployment-constraints)
  1. [Required Dependencies](#required-dependencies)
  1. [Diego Manifest Jobs](#diego-manifest-jobs)
1. [Pushing a CF Application to the Diego Runtime](#pushing-to-diego)
1. [Recommended Instance Types](#recommended-instance-types)
1. [Benchmarks](#benchmarks)

---

## <a name="deployment-examples"></a>Deployment Examples

### <a name="bosh-lite"></a>BOSH-Lite

To deploy CF and Diego to BOSH-Lite, follow the instructions at [examples/bosh-lite](examples/bosh-lite).


### <a name="aws"></a>AWS

In order to deploy Diego to AWS, follow the instructions in [examples/aws](examples/aws/README.md) to deploy BOSH, CF, and Diego to a new CloudFormation stack, or follow the instructions in [examples/aws](examples/minimal-aws/README.md) to deploy Diego alongside a [minimal CF deployment](https://github.com/cloudfoundry/cf-release/tree/master/example_manifests).


## <a name="additional-diego-resources"></a>Additional Diego Resources

  - The [Contribution Guidelines](CONTRIBUTING.md) describes the developer workflow for making changes to Diego.
  - The [Diego Design Notes](https://github.com/cloudfoundry/diego-design-notes) present an overview of Diego, and links to the various Diego components.
  - The [Migration Guide](https://github.com/cloudfoundry/diego-design-notes/blob/master/migrating-to-diego.md) describes how developers and operators can manage a transition from the DEAs to Diego.
  - The [Docker Support Notes](https://github.com/cloudfoundry/diego-design-notes/blob/master/docker-support.md) describe how Diego runs Docker-image-based apps in Cloud Foundry.
  - The [Diego-CF Compatibility Log](https://github.com/cloudfoundry/diego-cf-compatibility) records which versions of cf-release and diego-release are compatible, according to the Diego team's [automated testing pipeline](https://diego.ci.cf-app.com/?groups=diego).
  - [Supported Data Stores for Diego](docs/data-stores.md)
  - [Data Store Encryption](docs/data-store-encryption.md)
  - [TLS Configuration](docs/tls-configuration.md)
  - [Diego's Pivotal Tracker project](https://www.pivotaltracker.com/n/projects/1003146) shows what we're working on these days.
  - [Diego Metrics](docs/metrics.md) lists the various metrics that Diego emits through the Loggregator system.



---

## BOSH Dependencies

When deploying diego-release via BOSH, the following minimum versions are required:

* BOSH Release v255.4+ (Director version 1.3213.0)
* BOSH Stemcell 3125+

These versions ensure that the `pre-start` script in the rootfses job will be run
to extract and configure the cflinuxfs2 rootfs and that the drain scripts will
be called for all jobs on each VM during updates, instead of only the first
job. We also require `post-start` for the initial cell health check, which is
also provided by the same versions listed above.

---

## <a name="deploy-alongside-cf"></a>Deploying Alongside an Existing CF Deployment

Diego is typically deployed alongside a CF deployment to serve as its new container runtime. See "[Deploying Diego Alongside an Existing CF Deployment](docs/deploy-alongside-existing-cf.md)" for general instructions and guidelines to deploy Diego this way.

## <a name="release-compatibility"></a>Release Compatibility

Diego releases are tested against Cloud Foundry, Garden, and ETCD. Compatible versions
of Garden and ETCD are listed with Diego on the [Github releases page](https://github.com/cloudfoundry/diego-release/releases).

### Checking out a release of Diego

The Diego git repository is tagged with every release. To move the git repository
to match a release, do the following:

```bash
cd diego-release/
# checking out release v0.1437.0
git checkout v0.1437.0
./scripts/update
git clean -ffd
```

### From a final release of CF

On the CF Release [GitHub Releases](https://github.com/cloudfoundry/cf-release/releases) page,
recommended versions of Diego, Garden, and ETCD are listed with each CF Release.
This is the easiest way to correlate releases.

Alternatively, you can use records of CF and Diego compatibility captured from
automated testing. First look up the release candidate SHA for your CF release.
This is listed as the `commit_hash` in the release yaml file. Find the SHA in
[diego-cf-compatibility/compatibility-v2.csv](https://github.com/cloudfoundry/diego-cf-compatibility/blob/master/compatibility-v2.csv)
to look up tested versions of Diego Release, Garden, and ETCD.

Example: Let's say you want to deploy Diego alongside CF final release `222`. The release file
[`releases/cf-222.yml`](https://github.com/cloudfoundry/cf-release/blob/master/releases/cf-222.yml)
in the cf-release repository contains the line `commit_hash: 53014242`.
Finding `53014242` in `diego-cf-compatibility/compatibility-v2.csv` reveals Diego
0.1437.0, Garden 0.308.0, and ETCD 16 have been verified to be compatible.


### From a specific CF Release commit SHA

Not every cf-release commit will appear in the diego-cf compatibility table,
but many will work with some version of Diego.

If you can't find a specific cf-release SHA in the table, deploy the diego-release
that matches the most recent cf-release relative to that commit. To do this, go back
through cf-release's git log from your commit until you find a Final Release commit
and then look up that commit's SHA in the diego-cf compatibility table.

## <a name="manifest-generation"></a>Manifest Generation

The Diego manifest generation documentation can be found in [docs/manifest-generation.md](docs/manifest-generation.md).

## <a name="deployment-constraints"></a>Deployment Constraints

### <a name="required-dependencies"></a>Required Dependencies

Before deploying the Diego cluster, ensure that the consul server cluster it will connect to is already deployed. In most deployment scenarios, these consul servers come from a CF deployment.

Additionally, if configuring the BBS to use a relational data store such as a CF-MySQL database, that data store must be deployed or otherwise provisioned before deploying the Diego cluster.


### <a name="diego-manifest-jobs"></a>Diego Manifest Jobs

In your manifest, ensure that the following constraints on job update order and rate are met:

1. BBS servers should update before BBS clients. This can be achieved by placing `database_zN` instances at the beginning of the jobs list in your manifest. For example:

	```
	jobs:
	- instances: 1
	  name: database_z1
	```

1. `database_zN` nodes update one at a time. This can be achieved by setting `max_in_flight` to `1` and `serial` to `true` for `database_zN` jobs.

	```
	- instances: 1
	  name: database_z1
	  ...
	  update:
	    max_in_flight: 1
	    serial: true
	```

1. `brain_zN` jobs update separately from cells. This can be achieved by setting `max_in_flight` to `1` and `serial` to `true` for `brain_zN` jobs.

	```
	- instances: 1
	  name: brain_z1
	  ...
	  update:
	    max_in_flight: 1
	    serial: true
	```


## <a name="pushing-to-diego"></a>Pushing a CF Application to the Diego Runtime

1. Create and target a CF org and space:

  ```bash
  cf api --skip-ssl-validation api.bosh-lite.com
  cf auth admin admin
  cf create-org diego
  cf target -o diego
  cf create-space diego
  cf target -s diego
  ```

1. Change into your application directory and push your application without starting it:

  ```bash
  cd <app-directory>
  cf push my-app --no-start
  ```

1. [Enable Diego](https://github.com/cloudfoundry/diego-design-notes/blob/master/migrating-to-diego.md#targeting-diego) for your application.

1. Start your application:

  ```bash
  cf start my-app
  ```

---
## Recommended Instance Types

If you are deploying to AWS, you can use our recommended instance types by spiff merging
your `iaas-settings.yml` with our provided `manifest-generation/misc-templates/aws-iaas-settings.yml`:

```
spiff merge \
	manifest-generation/misc-templates/aws-iaas-settings.yml \
	/path/to/iaas-settings.yml \
	> /tmp/iaas-settings.yml
```

You can then use the template generated as the `iaas-settings.yml` for the `scripts/generate-deployment-manifest` tool.
The cell jobs currently use `r3.xlarge` as their `instance_type`.
For production deployments, we recommend that you increase the `ephemeral_disk` size.
This can be done by specifying the following in your `iaas-settings.yml` under the cell resource pool definitions:

```
ephemeral_disk:
  size: 174_080
  type: gp2
```

## Benchmarks

### Viewing Results

Diego benchmark results can be found
[here](http://time-rotor-diego-benchmarks.s3.amazonaws.com/). You can also
visit this [dashboard](https://p.datadoghq.com/sb/ed32fa2e4-a6d96f22f4) to view
the results for all benchmark runs in the last 24 hours.

Descriptions of the metrics from the benchmark runs are available in the
[BBS Benchmark documentation](https://github.com/cloudfoundry/benchmarkbbs#collected-metrics).

