---
title: SSH Daemon
expires_at : never
tags: [diego-release, diego-ssh]
---

## SSH Daemon

The ssh daemon is a lightweight implementation that is built around go's ssh
library. It supports command execution, interactive shells, local port
forwarding, scp, and sftp. The daemon is self-contained and has no
dependencies on the container root file system.

The daemon is focused on delivering basic access to application instances in
Cloud Foundry. It is intended to run as an unprivileged process and
interactive shells and commands will run as the daemon user. The daemon only
supports one authorized key is not intended to support multiple users.

The daemon can be made available on a file server and Diego LRPs that
want to use it can include a download action to acquire the binary and a run
action to start it. Cloud Foundry applications will download the daemon as
part of the lifecycle bundle.

[bridge]: https://github.com/cloudfoundry/diego-design-notes#cc-bridge-components
[cflinuxfs3]: https://github.com/cloudfoundry/cflinuxfs3
[cli]: https://github.com/cloudfoundry/cli
[non-standard-oauth-auth-code]: https://github.com/cloudfoundry/uaa/blob/master/docs/UAA-APIs.rst#api-authorization-requests-code-get-oauth-authorize-non-standard-oauth-authorize
