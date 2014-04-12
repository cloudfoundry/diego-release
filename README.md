# diego release

<p align="center">
  <img src="http://i.imgur.com/WrqaOd9.png" alt="Go Diego Go!" title="Go Diego Go!"/>
</p>

A BOSH release for deploying the following Diego components:

1. [Executor](https://github.com/cloudfoundry-incubator/executor)
1. [Warden-Linux](https://github.com/cloudfoundry-incubator/warden-linux)
1. [Stager](https://github.com/cloudfoundry-incubator/stager)
1. [File Server](https://github.com/cloudfoundry-incubator/file-server)

These components build out the new runtime architecture for Cloud Foundry,
replacing the DEA and Health Manager.

This release must be composed with another release to provide
[etcd](https://github.com/coreos/etcd) and
[NATS](https://github.com/apcera/gnatsd). In practice we always compose with
[cf-release](https://github.com/cloudfoundry/cf-release).
