# <a name="deprecations"></a>Deprecations

This document lists deprecated properties of the job templates in this BOSH release, metrics for Diego components, and API fields and endpoints.


## <a name="bosh-job-properties"></a>BOSH job properties

### <a name="bosh-job-properties-auctioneer"></a>`auctioneer`

| property                           | deprecated | removed | notes                                                  |
|------------------------------------|------------|---------|--------------------------------------------------------|
| `diego.auctioneer.bbs.require_ssl` | v1.35.0    | v2.1.0  | The BBS API now requires mutual TLS.                   |
| `diego.auctioneer.dropsonde_port`  | v1.35.0    | v2.1.0  | Loggregator API v1 is no longer supported in Diego v2. |
| `diego.auctioneer.rep.require_tls` | v2.1.0     | N/A     | Relevant only when upgrading from Diego v1.            |


### <a name="bosh-job-properties-bbs"></a>`bbs`

| property                                   | deprecated | removed | notes                                                                |
|--------------------------------------------|------------|---------|----------------------------------------------------------------------|
| `diego.bbs.auctioneer.api_url`             | v1.6.0     | v2.1.0  | Use `diego.bbs.auctioneer.api_location` instead.                     |
| `diego.bbs.desired_lrp_creation_timeout`   | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                             |
| `diego.bbs.dropsonde_port`                 | v1.35.0    | v2.1.0  | Loggregator API v1 is no longer supported in Diego v2.               |
| `diego.bbs.etcd.ca_cert`                   | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                             |
| `diego.bbs.etcd.client_cert`               | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                             |
| `diego.bbs.etcd.client_key`                | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                             |
| `diego.bbs.etcd.client_session_cache_size` | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                             |
| `diego.bbs.etcd.machines`                  | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                             |
| `diego.bbs.etcd.max_idle_conns_per_host`   | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                             |
| `diego.bbs.etcd.require_ssl`               | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                             |
| `diego.bbs.require_ssl`                    | v1.35.0    | v2.0.0  | The BBS API now requires mutual TLS.                                 |
| `diego.bbs.sql.db_connection_string`       | v0.1490.0  | v2.1.0  | Use `diego.bbs.sql.db_{host,port,schema,username,password}` instead. |
| `diego.bbs.auctioneer.require_tls`         | v2.1.0     | N/A     | Relevant only when upgrading from Diego v1.                          |
| `diego.bbs.rep.require_tls`:               | v2.1.0     | N/A     | Relevant only when upgrading from Diego v1.                          |


### <a name="bosh-job-properties-benchmark-bbs"></a>`benchmark-bbs`

| property                                       | deprecated | removed | notes                                                                    |
|------------------------------------------------|------------|---------|--------------------------------------------------------------------------|
| `benchmark-bbs.bbs.require_ssl`                | v1.35.0    | v2.0.0  | The BBS API now requires mutual TLS.                                     |
| `benchmark-bbs.etcd.ca_cert`                   | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                                 |
| `benchmark-bbs.etcd.client_cert`               | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                                 |
| `benchmark-bbs.etcd.client_key`                | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                                 |
| `benchmark-bbs.etcd.client_session_cache_size` | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                                 |
| `benchmark-bbs.etcd.machines`                  | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                                 |
| `benchmark-bbs.etcd.max_idle_conns_per_host`   | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                                 |
| `benchmark-bbs.etcd.require_ssl`               | v1.35.0    | v2.0.0  | ETCD is no longer supported in Diego v2.                                 |
| `benchmark-bbs.sql.db_connection_string`       | N/A        | v2.0.0  | Use `benchmark-bbs.sql.db_{host,port,schema,username,password}` instead. |


### <a name="bosh-job-properties-cfdot"></a>`cfdot`

| property                      | deprecated | removed | notes                              |
|-------------------------------|------------|---------|------------------------------------|
| `diego.cfdot.bbs.ca_cert`     | v1.31.1    | v2.1.0  | Use `tls.ca_certificate` instead.  |
| `diego.cfdot.bbs.client_cert` | v1.31.1    | v2.1.0  | Use `tls.certificate` instead.     |
| `diego.cfdot.bbs.client_key`  | v1.31.1    | v2.1.0  | Use `tls.private_key` instead.     |
| `diego.cfdot.bbs.use_ssl`     | v1.35.0    | v2.0.0  | BBS API now requires mutual TLS.   |


### <a name="bosh-job-properties-file-server"></a>`file_server`

| property                           | deprecated | removed | notes                                                  |
|------------------------------------|------------|---------|--------------------------------------------------------|
| `diego.file_server.dropsonde_port` | v1.35.0    | v2.1.0  | Loggregator API v1 is no longer supported in Diego v2. |


### <a name="bosh-job-properties-locket"></a>`locket`

| property         | deprecated | removed | notes                                                  |
|------------------|------------|---------|--------------------------------------------------------|
| `dropsonde_port` | v1.35.0    | v2.1.0  | Loggregator API v1 is no longer supported in Diego v2. |


### <a name="bosh-job-properties-rep"></a>`rep and rep_windows`

| property                                 | deprecated | removed | notes                                                                            |
|------------------------------------------|------------|---------|----------------------------------------------------------------------------------|
| `admin_api.require_tls`                  | v1.35.0    | v2.0.0  | The cell rep APIs now require mutual TLS.                                        |
| `diego.executor.ca_certs_for_downloads`  | v1.11.0    | v2.1.0  | Use `tls.ca_cert` instead.                                                       |
| `diego.executor.export_network_env_vars` | v1.35.0    | v2.1.0  | Always enabled in Diego v2.                                                      |
| `diego.rep.bbs.ca_cert`                  | v1.35.0    | v2.0.0  | Use `tls.ca_cert` instead.                                                       |
| `diego.rep.bbs.client_cert`              | v1.35.0    | v2.0.0  | Use `tls.cert` instead.                                                          |
| `diego.rep.bbs.client_key`               | v1.35.0    | v2.0.0  | Use `tls.key` instead                                                            |
| `diego.rep.bbs.require_ssl`              | v1.35.0    | v2.0.0  | The BBS API now requires mutual TLS.                                             |
| `diego.rep.ca_cert`                      | v1.35.0    | v2.0.0  | Use `tls.ca_cert` instead.                                                       |
| `diego.rep.dropsonde_port`               | v1.35.0    | v2.1.0  | Loggregator API v1 is no longer supported in Diego v2.                           |
| `diego.rep.enable_legacy_api_endpoints`  | v1.35.0    | v2.1.0  | Diego v2 removes these endpoints from the admin API listener.                    |
| `diego.rep.listen_addr`                  | v1.35.0    | v2.1.0  | Use `diego.rep.listen_addr_admin` and `diego.rep.listen_addr_securable` instead. |
| `diego.rep.require_tls`                  | v1.35.0    | v2.0.0  | The cell rep APIs now require mutual TLS.                                        |
| `diego.rep.server_cert`                  | v1.35.0    | v2.0.0  | Use `tls.cert` instead.                                                          |
| `diego.rep.server_key`                   | v1.35.0    | v2.0.0  | Use `tls.key` instead.                                                           |
| `diego.rep.trusted_certs`                | v1.30.0    | v2.1.0  | Use `containers.trusted_ca_certificates` instead.                                |
| `use_v2_tls`                             | v1.35.0    | v2.0.0  | The cell rep APIs now require mutual TLS.                                        |


### <a name="bosh-job-properties-route-emitter"></a>`route_emitter and route_emitter_windows`


| property                              | deprecated | removed | notes                                                  |
|---------------------------------------|------------|---------|--------------------------------------------------------|
| `diego.route_emitter.bbs.require_ssl` | v1.35.0    | v2.0.0  | The BBS API now requires mutual TLS.                   |
| `diego.route_emitter.dropsonde_port`  | v1.35.0    | v2.1.0  | Loggregator API v1 is no longer supported in Diego v2. |


### <a name="bosh-job-properties-ssh-proxy"></a>`ssh_proxy`

| property                          | deprecated | removed | notes                                                  |
|-----------------------------------|------------|---------|--------------------------------------------------------|
| `diego.ssh_proxy.bbs.require_ssl` | v1.35.0    | v2.0.0  | The BBS API now requires mutual TLS.                   |
| `diego.ssh_proxy.dropsonde_port`  | v1.35.0    | v2.1.0  | Loggregator API v1 is no longer supported in Diego v2. |
| `diego.ssh_proxy.uaa_token_url`   | v1.32.1    | v2.1.0  | Use `diego.ssh_proxy.uaa.url` instead.                 |


### <a name="bosh-job-properties-vizzini"></a>`vizzini`

| property                  | deprecated | removed | notes                                |
|---------------------------|------------|---------|--------------------------------------|
| `vizzini.bbs.require_ssl` | v1.35.0    | v2.0.0  | The BBS API now requires mutual TLS. |


## <a name="component-metrics"></a>Component metrics

### <a name="component-metrics-rep"></a>`rep` and `rep_windows`

- `GardenContainerCreationDuration`: Deprecated in favor of `GardenContainerCreationFailedDuration` and `GardenContainerCreationSucceededDuration`.

### <a name="component-metrics-route-emitter"></a>`route_emitter`

- `MessagesEmitted`: Deprecated in favor of `HTTPRouteNATSMessagesEmitted` and `InternalRouteNATSMessagesEmitted`.

## <a name="component-apis"></a>Component APIs

The [BBS API docs](https://github.com/cloudfoundry/bbs/tree/master/doc) and [routes](https://github.com/cloudfoundry/bbs/blob/master/routes.go) list the currently deprecated fields and endpoints inline. The current standard practice in Diego is to retain deprecated API fields and endpoints for at least a full major version of the release for cross-version compatibility.

### BBS

#### Endpoints

| endpoint                                  | deprecated       | removed | notes                                                    |
| --------------------------                | ----------       | ------- | ------------------------------------                     |
| `/v1/desired_lrps/list.r2`                | v2.20.0          | N/A     | Use `/v1/desired_lrps/list.r3` instead.                  |
| `/v1/desired_lrps/get_by_process_guid.r2` | v2.20.0          | N/A     | Use `/v1/desired_lrps/get_by_process_guid.r3` instead.   |
| `/v1/tasks/fail`                          | v2.27.0          | N/A     | Use `/v1/tasks/complete` and `/v1/tasks/cancel` instead. |
| `/v1/tasks/get_by_task_guid.r2`           | v2.20.0          | N/A     | Use `/v1/tasks/get_by_task_guid.r3` instead.             |
| `/v1/tasks/list.r2`                       | v2.20.0          | N/A     | Use `/v1/tasks/list.r3` instead.                         |
| `/v1/events`                              | v2.20.0          | N/A     | Use `/v1/events.r1` instead.                             |
| `/v1/events/tasks`                        | v2.20.0          | N/A     | Use `/v1/events/tasks.r1` instead.                       |
| `/v1/events/lrp_instances`                | v2.20.0          | N/A     | Use `/v1/events/lrp_instances.r1` instead.               |

#### Fields

- [DesiredLRP::deprecated_start_timeout_s](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/desired_lrp.proto#L88)
- [TimeoutAction::deprecated_timeout_ns](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/actions.proto#L62)
- [VolumeMount::deprecated_volume_id](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/volume_mount.proto#L20)
- [VolumeMount::deprecated_mode](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/volume_mount.proto#L21)
- [VolumeMount::deprecated_config](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/volume_mount.proto#L22)
- [`ImageLayer::DigestAlgorithm` deprecated algorithm: SHA512](https://github.com/cloudfoundry/bbs/blob/808072216b1ae29e691336057ed0871ee84ab905/models/image_layer.proto#L11)

## <a name="docker-registries-supporting-v2s1-manifests"></a>Docker Registries Supporting v2 schema 1 manifests

Support for running LRPs using Docker images from registries that serve only [v2 schema 1 manifests](https://docs.docker.com/registry/spec/manifest-v2-1/) is deprecated and will be removed in 3.0.0. Docker registries should be updated to serve [v2 schema2 manifests](https://docs.docker.com/registry/spec/manifest-v2-2/).

