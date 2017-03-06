# Required BOSH Versions

Deploying diego-release requires the following minimum versions of BOSH dependencies:

* BOSH release v255.4+ (Director version 1.3213.0).
* BOSH stemcell 3263+.
* garden-runc 1.2.0+
* grootfs-release v0.11.0+

These versions ensure that the following 

- Drain scripts are called for all jobs on each VM during updates.
- `post-start` scripts are called for the `bbs` and `rep` Diego jobs.
- `pre-start` scripts are called for the `cflinuxfs2-rootfs-setup` job co-located on the Diego cell instances.
- The `Image` field is available in Garden's `ContainerSpec`.
- Diego is able to run Docker images that require authentication.