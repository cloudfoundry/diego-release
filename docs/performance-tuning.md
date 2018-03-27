# Performance Tuning Recommendations

This document describes recommendations for performance tuning of the Diego Data Store.


### Table of Contents

1. [BBS Performance Tuning](#bbs-tuning)
1. [Locket Performance Tuning](#locket-tuning)
2. [MySQL Performance Tuning](#mysql-performance-tuning)


### <a name="bbs-tuning"></a> BBS Performance Tuning

Maximum number of BBS connections to the database can be set using the `diego.bbs.sql.max_open_connections`. The maximum number of Idle connections can be set using `diego.bbs.sql.max_idle_connections`. By default `diego.bbs.sql.max_idle_connections` is set to the same value as `diego.bbs.sql.max_open_connections` to avoid recreating connections to the database uneccesarily.

### <a name="locket-tuning"></a> Locket Performance Tuning

Maximum number of BBS connections to the database can be set using the `database.max_open_connections`. Unlike the BBS, the maximum number of Idle connections cannot be set. It is always set to the same value as `database.max_open_connections`

### <a name="mysql-performance-tuning"></a> MySQL Performance Tuning

#### Total number of SQL connections

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
