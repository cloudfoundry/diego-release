# <a name="deprecations"></a>Deprecations

This document lists deprecated properties of the job templates in this BOSH release, metrics for Diego components, and API fields and endpoints.


## <a name="bosh-job-properties"></a>BOSH job properties

### <a name="bosh-job-properties-auctioneer"></a>`auctioneer`

- `diego.auctioneer.rep.require_tls:`: Deprecated in v2. TLS is now required for the Rep in Diego v2.0+.
- `diego.auctioneer.bbs.require_ssl`: Removed in v2. TLS is now required for the BBS in Diego v2.0+.
- `diego.auctioneer.dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.


### <a name="bosh-job-properties-bbs"></a>`bbs`

- `diego.bbs.auctioneer.require_tls`: Deprecated in v2. TLS is now required for the Auctioneer in Diego v2.0+.
- `diego.bbs.rep.require_tls:`: Deprecated in v2. TLS is now required for the Rep in Diego v2.0+.
- `diego.bbs.auctioneer.api_url`: Removed in v2 in favor of `diego.bbs.auctioneer.api_location`.
- `diego.bbs.dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.
- `diego.bbs.desired_lrp_creation_timeout`: Removed in v2 as part of removing support for etcd.
- `diego.bbs.etcd.*`: Removed in v2 as part of removing support for etcd.
- `diego.bbs.require_ssl`: Removed in v2. TLS is required for the BBS in Diego v2.0+.
- `diego.bbs.sql.db_connection_string`: Removed in v2 in favor of the other `diego.bbs.sql.db_*` properties.


### <a name="bosh-job-properties-benchmark-bbs"></a>`benchmark-bbs`

- `benchmark-bbs.bbs.require_ssl`: Removed in v2. TLS will be required for the BBS in Diego v2.0+.
- `benchmark-bbs.etcd.*`: Removed in v2. ETCD will no longer be supported in Diego v2.0.


### <a name="bosh-job-properties-cfdot"></a>`cfdot`

- `diego.cfdot.bbs.ca_cert`: Removed in v2 in favor of `tls.ca_certificate`.
- `diego.cfdot.bbs.client_cert`: Removed in v2 in favor of `tls.certificate`.
- `diego.cfdot.bbs.client_key`: Removed in v2 in favor of `tls.private_key`.
- `diego.cfdot.bbs.use_ssl`: Removed in v2. TLS will be required for the BBS in Diego v2.0+.


### <a name="bosh-job-properties-file-server"></a>`file_server`

- `diego.file_server.dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.


### <a name="bosh-job-properties-locket"></a>`locket`

- `dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.


### <a name="bosh-job-properties-rep"></a>`rep`

- `admin_api.require_tls`: Removed in v2. Mutual TLS will be required in v2.0+.
- `diego.executor.ca_certs_for_downloads`: Removed in v2 in favor of `tls.ca_cert`.
- `diego.executor.export_network_env_vars`: Removed in v2. Network environment variables will always be exported in Diego v2.0+.
- `diego.rep.listen_addr`: Removed in v2 in favor of `diego.rep.listen_addr_admin` and `diego.rep.listen_addr_securable`.
- `diego.rep.dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.
- `diego.rep.enable_legacy_api_endpoints`: Removed in v2 since the legacy API server will be removed in Diego v2.0.
- `diego.rep.require_tls`: Removed in v2 since mutual TLS will be required in Diego v2.0+.
- `diego.rep.trusted_certs`: Removed in v2 in favor of `containers.trusted_ca_certificates`.
- `diego.rep.bbs.require_ssl`: Removed in v2, TLS will be required for the BBS in Diego v2.0+.


### <a name="bosh-job-properties-rep-windows"></a>`rep_windows`

- `admin_api.require_tls`: Removed in v2. Mutual TLS will be required in v2.0+.
- `diego.executor.ca_certs_for_downloads`: Removed in v2 in favor of `tls.ca_cert`.
- `diego.executor.export_network_env_vars`: Removed in v2. Network environment variables will always be exported in Diego v2.0+.
- `diego.rep.listen_addr`: Removed in v2 in favor of `diego.rep.listen_addr_admin` and `diego.rep.listen_addr_securable`.
- `diego.rep.dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.
- `diego.rep.enable_legacy_api_endpoints`: Removed in v2 since the legacy API server will be removed in Diego v2.0.
- `diego.rep.require_tls`: Removed in v2 since mutual TLS will be required in Diego v2.0+.
- `diego.rep.trusted_certs`: Removed in v2 in favor of `containers.trusted_ca_certificates`.
- `diego.rep.bbs.require_ssl`: Removed in v2, TLS will be required for the BBS in Diego v2.0+.


### <a name="bosh-job-properties-route-emitter"></a>`route_emitter`

- `diego.route_emitter.bbs.require_ssl`: Removed in v2. TLS will be required for the BBS in Diego v2.0+.
- `diego.route_emitter.dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.


### <a name="bosh-job-properties-route-emitter-windows"></a>`route_emitter_windows`

- `diego.route_emitter.bbs.require_ssl`: Removed in v2. TLS will be required for the BBS in Diego v2.0+.
- `diego.route_emitter.dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.


### <a name="bosh-job-properties-ssh-proxy"></a>`ssh_proxy`

- `diego.ssh_proxy.bbs.require_ssl`: Removed in v2, TLS will be required for the BBS in Diego v2.0+.
- `diego.ssh_proxy.dropsonde_port`: Removed in v2 as part of removing support for loggregator API v1.
- `diego.ssh_proxy.uaa_token_url`: Removed in v2 in favor of `diego.ssh_proxy.uaa.url`.


### <a name="bosh-job-properties-vizzini"></a>`vizzini`

- `vizzini.bbs.require_ssl`: Removed in v2. TLS will be required for the BBS in Diego v2.0+.


## <a name="component-metrics"></a>Component metrics

### <a name="component-metrics-rep"></a>`rep` and `rep_windows`

- `GardenContainerCreationDuration`: Deprecated in favor of `GardenContainerCreationFailedDuration` and `GardenContainerCreationSucceededDuration`.


### <a name="component-metrics-route-emitter"></a>`route_emitter`

- `MessagesEmitted`: Deprecated in favor of `HTTPRouteNATSMessagesEmitted` and `InternalRouteNATSMessagesEmitted`.


## <a name="component-apis"></a>Component APIs

The [BBS API docs](https://github.com/cloudfoundry/bbs/tree/master/doc) and [routes](https://github.com/cloudfoundry/bbs/blob/master/routes.go) list the currently deprecated fields and endpoints inline. The current standard practice in Diego is to retain deprecated API fields and endpoints for at least a full major version of the release for cross-version compatibility.

### BBS

#### Endpoints

- `/v1/desired_lrps/list.r1` Method: `POST`
- `/v1/desired_lrps/get_by_process_guid.r1` Method: `POST`
- `/v1/desired_lrps/list` Method: `POST`
- `/v1/desired_lrps/get_by_process_guid` Method: `POST`
- `/v1/desired_lrp/desire.r1` Method: `POST`
- `/v1/desired_lrp/desire` Method: `POST`
- `/v1/tasks/list.r1` Method: `POST`
- `/v1/tasks/list` Method: `POST`
- `/v1/tasks/get_by_task_guid.r1` Method: `POST`
- `/v1/tasks/get_by_task_guid` Method: `GET`
- `/v1/tasks/desire.r1` Method: `POST`
- `/v1/tasks/desire` Method: `POST`
- `/v1/cells/list.r1` Method: `GET`

**Note** `POST` requests to `/v1/cells/list.r1` are **NOT** deprecated

#### Fields

- [DesiredLRP::deprecated_start_timeout_s](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/desired_lrp.proto#L88)
- [TimeoutAction::deprecated_timeout_ns](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/actions.proto#L62)
- [VolumeMount::deprecated_volume_id](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/volume_mount.proto#L20)
- [VolumeMount::deprecated_mode](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/volume_mount.proto#L21)
- [VolumeMount::deprecated_config](https://github.com/cloudfoundry/bbs/blob/e2ecd53354162c7ba39cb16fcd73e0830041bc11/models/volume_mount.proto#L22)
