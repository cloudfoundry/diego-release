# <a name="deprecations"></a>Deprecations

This document lists deprecated properties of the job templates in this BOSH release, metrics for Diego components, and API fields and endpoints.


## <a name="bosh-job-properties"></a>BOSH job properties

### <a name="bosh-job-properties-bbs"></a>`bbs`

- `diego.bbs.auctioneer.api_url`: Deprecated in favor of `diego.bbs.auctioneer.api_location`.
- `diego.bbs.sql.db_connection_string`: Deprecated in favor of the other `diego.bbs.sql.db_*` properties.


### <a name="bosh-job-properties-cfdot"></a>`cfdot`

- `diego.cfdot.bbs.ca_cert`: Deprecated in favor of `tls.ca_certificate`.
- `diego.cfdot.bbs.client_cert`: Deprecated in favor of `tls.certificate`.
- `diego.cfdot.bbs.client_key`: Deprecated in favor of `tls.private_key`.


### <a name="bosh-job-properties-rep"></a>`rep`

- `diego.executor.ca_certs_for_downloads`: Deprecated in favor of `tls.ca_cert`.
- `diego.rep.trusted_certs`: Deprecated in favor of `containers.trusted_ca_certificates`.


### <a name="bosh-job-properties-rep-windows"></a>`rep_windows`

- `diego.executor.ca_certs_for_downloads`: Deprecated in favor of `tls.ca_cert`.
- `diego.rep.trusted_certs`: Deprecated in favor of `containers.trusted_ca_certificates`.


### <a name="bosh-job-properties-ssh-proxy"></a>`ssh_proxy`

- `diego.ssh_proxy.uaa_token_url`: Deprecated in favor of `diego.ssh_proxy.uaa.url`.


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
