# Scaling and Performance Tuning Recommendations

This document describes recommendations for performance tuning of the Diego Data Store.


## Table of Contents

1. [Component scaling guidelines](#component-scaling-guidelines)
1. [BBS Performance Tuning](#bbs-tuning)
1. [Locket Performance Tuning](#locket-tuning)
2. [MySQL Performance Tuning](#mysql-performance-tuning)


## <a name="#component-scaling-guidelines"/> Component Scaling Guidelines

### Scaling vertically

The following components must be scaled vertically (more cores and/or memory). Horizontally scaling them does not make sense since there is only one instance active at any given point in time:

1. `route_emitter` (only when running in global mode as opposed to local mode)
2. `bbs`
3. `auctioneer`

### Scaling horizontally

The following components can be scaled horizontally as well as vertically:

1. `file_server`
2. `locket`
3. `rep`
4. `rep_windows`
5. `route_emitter` (when running in local mode)
6. `route_emitter_windows` (when running in local mode)
7. `ssh_proxy`

### Job specific guidelines

The following jobs require careful planning:

- `bbs`
  1. It is **NOT** recommended to use a [burstable performance](https://aws.amazon.com/ec2/instance-types/) VMs, for example AWS' `t2` instances
  2. The performance of the BBS highly depends on the performance of the SQL backend. A less performant SQL backend could reduce the throughput and increase the latency of the BBS requests.
  3. The performance of the BBS is inversely proportional to the total number of ActualLRPs running
  4. We currently benchmark the BBS on a VM with 16 cpu cores and 60GB memory. The SQL backend have the same number of cores and memory. This setup can handle load from 1000 cells (running `rep` & `route-emitter`) with a total of 250K LRPs.
- `rep`:
  1. Although the `rep` is a horizontally scalable component, the resourcese of each `rep` (which usually run on an instance-group named `diego-cell` along with the [garden](http://bosh.io/jobs/garden?source=github.com/cloudfoundry/garden-runc-release) job) will affect the total number of containers that can run on each cell. For example, if the `rep` is running on a VM with 20GB of memory, it can only run 20 ActualLRPs with a 1GB memory limit. This also applies to available disk space.
  2. In case a larger cell is not applicable, an operator can overcommit memory and disk by setting the following properties on the `rep` job:
     1. `diego.executor.memory_capacity_mb`
     2. `diego.executor.disk_capacity_mb`
- `locket`:
  1. It is **NOT** recommended to use a [burstable performance](https://aws.amazon.com/ec2/instance-types/) VMs, for example AWS' `t2` instances
  2. The performance of the Locket highly depends on the performance of the SQL backend. A less performant SQL backend could reduce the throughput and increase the latency of the Locket requests.
  3. We currently benchmark the Locket on a VM with 16 cpu cores and 60GB memory. The SQL backend have the same number of cores and memory. This setup can handle 1000 cells (running `rep` & `route-emitter`) with a total of 250K LRPs.
  4. **Note** Although `locket` is horizontally scalable job. It is usually deployed on the `diego-api` instance group along with `bbs`. If that is the case we recommend to scale the instance-group vertically

## <a name="bbs-tuning"></a> BBS Performance Tuning

Maximum number of BBS connections to the database can be set using the `diego.bbs.sql.max_open_connections`. The maximum number of Idle connections can be set using `diego.bbs.sql.max_idle_connections`. By default `diego.bbs.sql.max_idle_connections` is set to the same value as `diego.bbs.sql.max_open_connections` to avoid recreating connections to the database uneccesarily.

## <a name="locket-tuning"></a> Locket Performance Tuning

Maximum number of BBS connections to the database can be set using the `database.max_open_connections`. Unlike the BBS, the maximum number of Idle connections cannot be set. It is always set to the same value as `database.max_open_connections`

## <a name="mysql-performance-tuning"></a> MySQL Performance Tuning

### Total number of SQL connections

The maximum number of Diego connections to the SQL backend can be calculated using the following formula:

`<diego.bbs.sql.max_idle_connections>  + <database.max_open_connections> * <diego-api instances>`

- Since only one BBS instance is active we don't need to multiply `diego.bbs.sql.max_open_connections` by the number of instances
- If other components connect to the SQL database you will need to add their maximum number of connections to get an accurate figure
- The above is the maximum number of connections from BBS & Locket to the SQL backend. It doesn't mean that BBS and/or Locket will always make that many connections to the SQL backend.

Operators can use the following [CF Deployment](https://github.com/cloudfoundry/cf-deployment)
[Operations File](http://bosh.io/docs/cli-ops-files.html) to tune MySQL
configuration for high traffic deployment: [mysql.yml](../operations/benchmarks/mysql.yml)

Operators may freely change the sizing and scaling parameters provided in the
[benchmark operations files](../operations/benchmarks/).
