---
title: Scaling and Performance Tuning Recommendations
expires_at : never
tags: [diego-release]
---

# Scaling and Performance Tuning Recommendations

This document describes recommendations for performance tuning of the Diego
Data Store.


## Table of Contents

1. [Component scaling guidelines](#component-scaling-guidelines)
1. [Foundation Level Tuning](#foundation-level-tuning)
1. [Locket Performance Tuning](#locket-tuning)
1. [SQL Performance Tuning](#sql-performance-tuning)
1. [Compensating for Envoy memory overhead](#envoy-proxy-configuration.md)
1. [VM Size Recommendation](#vm-size-recommendation)

## <a name="component-scaling-guidelines"/> Component Scaling Guidelines

### Scaling vertically

The following components must be scaled vertically (more CPU cores and/or
memory) after having scaled to at least 3 instances horizontally. After 3
instances, scaling them horizontally does not make sense since there is only
one instance active at any given point in time.

1. `auctioneer` - [metrics to
   monitor](../jobs/auctioneer/templates/indicators.yml.erb)
1. `bbs` - [metrics to monitor](../jobs/bbs/templates/indicators.yml.erb)
1. `route_emitter` (only when running in global mode as opposed to cell-local
   mode)

### Scaling horizontally

The following components can be scaled horizontally as well as vertically:

1. `file_server`
1. `locket` - [metrics to monitor](../jobs/locket/templates/indicators.yml.erb)
1. `rep` - [metrics to monitor](../jobs/rep/templates/indicators.yml.erb)
1. `rep_windows` - [metrics to
   monitor](../jobs/rep_windows/templates/indicators.yml.erb)
1. `route_emitter` (only when running in cell-local mode)
1. `route_emitter_windows` (only when running in cell-local mode)
1. `ssh_proxy`

### Job specific guidelines

The following jobs require more considered planning:

- `bbs`:
  1. It is **NOT** recommended to use [burstable
  performance](https://aws.amazon.com/ec2/instance-types/) VMs, such as AWS
  `t2`-family instances.
  1. The performance of the BBS depends significantly on the performance of its
  SQL database. A less performant SQL backend could reduce the throughput and
  increase the latency of the BBS requests.
  1. The BBS activity from API request load and internal activity are both
  directly proportional to the total number of running app instances (or
  running ActualLRPs, in pure Diego terms). If the number of instances that the
  deployment supports increases without a corresponding increase in VM
  resources, BBS API response times may increase instead.
- `rep`:
  1. Although the `rep` is a horizontally scalable component, the resources
  available to each `rep` on its VM (typically called a "Diego cell") affect
  the total number of app instance and task containers that can run on that VM.
  For example, if the `rep` is running on a VM with 20GB of memory, it can only
  run 20 app instances that each have a 1-GB memory limit. This constraint also
  applies to available disk capacity.
  1. In case it is not possible for an operator to deploy larger cell VMs or to
  increase the number of cell VMs, an operator can overcommit memory and disk
  by setting the following properties on the `rep` job:
     1. `diego.executor.memory_capacity_mb`
     1. `diego.executor.disk_capacity_mb` Operators that overcommit cell
     capacity should be extremely careful not to run out of physical memory or
     disk capacity on the cells.
- `locket`:
  1. It is **NOT** recommended to use [burstable
  performance](https://aws.amazon.com/ec2/instance-types/) VMs, such as AWS
  `t2`-family instances.
  1. The performance of the Locket instances depends significantly on the
  performance of its SQL database. A less performant SQL backend could reduce
  the throughput and increase the latency of the Locket requests, which may in
  turn affect the availability of services such as the BBS, the auctioneer, and
  the cell reps that maintain locks and presences in Locket.
  1. **Note**: Although `locket` is a horizontally scalable job, in
  [cf-deployment](https://github.com/cloudfoundry/cf-deployment) it is deployed
  on the `diego-api` instance group along with the `bbs` job. In that case we
  recommend still to scale the instance group vertically.

The Diego team currently benchmarks the BBS and Locket together on a VM with 16
CPU cores and 60GB memory. The MySQL and Postgres backends have the same number
of cores and memory. This setup can handle load from 1000 simulated cells
(running `rep` and `route-emitter`) with a total of 250K LRPs.

## <a name="foundation-level-tuning"></a> Foundation Level Tuning
1. [Tips for Large Deployments with CF Networking and Silk
   Release](https://github.com/cloudfoundry/cf-networking-release/blob/a14fd8e9eaa46f399be15ad3b298c3be7e3359ab/docs/large_deployments.md)
1. [ARP Cache Limit for Large
   foundations](https://community.pivotal.io/s/article/5000e00001k5E3I1591986522237?language=en_US)
1. [Known Loggregator Scaling
   Issues](https://community.pivotal.io/s/article/known-loggregator-scaling-issues?language=en_US)


## <a name="bbs-tuning"></a> BBS Performance Tuning

The maximum number of connections from the active BBS to the SQL database can
be set using the `diego.bbs.sql.max_open_connections` property on the `bbs`
job, and the maximum number of idle connections can be set using
`diego.bbs.sql.max_idle_connections`. By default
`diego.bbs.sql.max_idle_connections` is set to the same value as
`diego.bbs.sql.max_open_connections` to avoid recreating connections to the
database uneccesarily.

## <a name="locket-tuning"></a> Locket Performance Tuning

The maximum number of connections from each Locket instance to the database can
be set using the `database.max_open_connections` property on the `locket` job.
Unlike the BBS, the Locket job does not permit the maximum number of idle
connections to be set independently, and always sets it to the same value as
`database.max_open_connections`.

## <a name="sql-performance-tuning"></a> SQL Performance Tuning

### Total number of SQL connections

In a [cf-deployment](https://github.com/cloudfoundry/cf-deployment)-based CF
cluster, an operator can the maximum number of connections from Diego
components (BBS and Locket) to the SQL backend using the following formula:

``` <diego.bbs.sql.max_open_connections> + <database.max_open_connections> *
<number of diego-api instances> ```

- The `diego.bbs.sql.max_open_connections` parameter contributes only once
  because there is only one active BBS instance.
- The actual number of active connections may be significantly lower than this
  maximum, depending on the scale of the app workload that the CF cluster
  supports.
- If other components connect to the same SQL database you will need to add
  their maximum number of connections to get an accurate figure.

### SQL deployment configuration

Operators can use the following
[cf-deployment](https://github.com/cloudfoundry/cf-deployment)-compatible
[operations files](http://bosh.io/docs/cli-ops-files.html) to tune their MySQL
or Postgres databases to support a large CF cluster:

- MySQL: [mysql.yml](../operations/benchmarks/mysql.yml)
- Postgres: [postgres.yml](../operations/benchmarks/postgres.yml)

These operations files are the ones used in the Diego team's 250K-instance
benchmark tests, and operators may freely change the sizing and scaling
parameters in them to match the resource needs of their own CF clusters.
```

## <a name="vm-size-recommendation"/> VM Size Recommendation

### BBS & Locket
10K app instances: Minimum 7.5GB Memory and 2vCPU
20K app instances: Minimum 15GB Memory and 4vCPU

### Auctioneer
10k app instances: Minimum 3.75GB memory and 1vCPU
