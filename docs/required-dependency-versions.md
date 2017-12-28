# Required Dependency Versions

The [release notes](https://github.com/cloudfoundry/diego-release/releases) also contain recommended versions of dependency releases for specific versions of Diego.


## BOSH Director and Stemcells

Deploying diego-release requires the following minimum versions of BOSH dependencies:

- BOSH release v261+.
- BOSH stemcell 3263+.

These BOSH versions ensure the following BOSH operations occur on the Diego jobs:

- Drain scripts are called for all jobs on each VM during updates.
- The `post-start` scripts are called for the `bbs` and `rep` Diego jobs.
- The `pre-start` script is called for the `cflinuxfs2-rootfs-setup` job co-located on the Linux Diego cells in a typical CF deployment.
- The `spec.id` field is populated in the Diego job templates.


## Garden releases

Diego-release also requires the following versions of supported Garden releases:

- Linux cells require garden-runc v1.2.0+. If declarative healthchecks are enabled, Diego requires garden-runc 1.10.0+.
- Windows cells require garden-windows v0.3.0+.

Deploying garden-runc 1.2.0+ and garden-Windows v0.3.0+ ensures the following features are available:

- The `Image` field is available on the `ContainerSpec` structure in the Garden API.
- The `NetIn` and `NetOut` fields are available on the `ContainerSpec` structure in the Garden API.

Additionally, garden-runc 1.10.0+ ensures the following:

- The `Image` field is available on the `ProcesSpec`. Diego uses this field to run the long-running declarative health-check process in a separate container.

For Linux Diego cells to be able to run containers based on Docker images that require authentication, garden-runc must be deployed with grootfs-release v0.11.0+.


## MySQL and Postgres

See the [Data Stores document](data-stores.md) for the minimum required versions of MySQL and Postgres and related BOSH releases.
