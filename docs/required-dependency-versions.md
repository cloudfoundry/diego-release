# Required Dependency Versions

The [release notes](https://github.com/cloudfoundry/diego-release/releases) also contain recommended versions of dependency releases for specific versions of Diego.


## BOSH Director and Stemcells

Deploying diego-release requires the following minimum versions of BOSH dependencies:

- BOSH release v264.7.0+.
- BOSH stemcell 3541+.

These BOSH versions ensure the following BOSH operations occur on the Diego jobs:

- Drain scripts are called for all jobs on each VM during updates.
- The `post-start` scripts are called for the `bbs` and `rep` Diego jobs.
- The `pre-start` script is called for the `cflinuxfs3-rootfs-setup` job co-located on the Linux Diego cells in a typical CF deployment.
- The `spec.id` field is populated in the Diego job templates.


## Garden releases

Diego-release also requires the following versions of supported Garden releases:

- Linux cells require garden-runc v1.11.1+.
- Windows cells require garden-windows v0.13.0+.

Deploying garden-runc 1.11.1+ and garden-windows v0.13.0+ ensures the following features are available:

- The `Image` field is available on the `ContainerSpec` structure in the Garden API.
- The `NetIn` and `NetOut` fields are available on the `ContainerSpec` structure in the Garden API.

Additionally, garden-runc 1.11.1+ ensures the following:

- The `Image` field is available on the `ProcessSpec`. Diego uses this field to run the long-running declarative health-check process in a separate container.


## MySQL and Postgres

See the [Data Stores document](data-stores.md) for the minimum required versions of MySQL and Postgres and related BOSH releases.
