# Required BOSH Versions

Deploying diego-release requires the following minimum versions of BOSH dependencies:

- BOSH release v255.4+ (Director version 1.3213.0).
- BOSH stemcell 3263+.

These BOSH versions ensure the following BOSH lifecycle management operations occur on the Diego jobs:

- Drain scripts are called for all jobs on each VM during updates.
- The `post-start` scripts are called for the `bbs` and `rep` Diego jobs.
- The `pre-start` script is called for the `cflinuxfs2-rootfs-setup` job co-located on the Linux Diego cells.
- The `spec.id` field is populated in the Diego job templates.


Diego-release also requires the following versions of supported Garden releases:

- For Linux cells: garden-runc 1.2.0+.
- For Windows cells: garden-windows v0.3.0+.

These versions ensure the following:

- The `Image` field is available on the `ContainerSpec` structure in the Garden API.
- The `NetIn` and `NetOut` fields are available on the `ContainerSpec` structure in the Garden API.

For Linux Diego cells to be able to run containers based on Docker images that require authentication, the garden-runc release must be deployed with grootfs-release v0.11.0+.
