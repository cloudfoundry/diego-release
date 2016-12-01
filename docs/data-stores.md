# Data Stores for Diego

This document describes the different types of data store supported by Diego across its different versions.

### Table of Contents

1. [Supported Data Stores](#supported-data-stores)
1. [Choosing a Relational Data Store Deployment](#choosing-relational-datastore-deployment)


### <a name="supported-data-stores"></a>Supported Data Stores

At present, Diego supports the following types of data store to back the BBS API server:

* Relational database: MySQL or PostgreSQL

#### etcd

Support for ETCD is now deprecated. Please see the next section on how to
configure Diego to use relational database as the backend.

#### Relational Databases

Diego supports two SQL dialects, MySQL and PostgreSQL, when configured to use a
relational data store to back the BBS. Official support for them will start
with a forthcoming Diego release version.

#### Migration of BBS Data from etcd to SQL

For a Diego deployment that has previously been configured to use etcd as its data store, configuring it also to connect to a relational store will cause the BBS to migrate the etcd data to the relational store automatically. After the migration, the data in etcd is marked as invalid, and Diego will not revert to using its data without manual intervention in the etcd key-value store to reset the version.

Converting Diego from standalone etcd to standalone relational requires two deploys:

1. Configure Diego to connect both to etcd and to the relational store, as with the `-s` flag to the [manifest-generation script](./manifest-generation.md).
2. Deploy Diego and verify that the BBS nodes have migrated the etcd data to the relational store.
   1. Verifying BBS is using SQL backend
      ```shell
      bosh -d /path/to/diego.yml instances | grep database | awk '{print $2}' | xargs -n1 -P5 -I{} bosh -d /path/to/diego.yml ssh {} "grep bbs.migration-manager.finished-migrations /var/vcap/sys/log/bbs/bbs.stdout" 2>&1 | grep migration | grep -v Executing | wc -l
      ```

      Or manually by sshing into all database_z* vms and running the following:
      ```shell
      grep bbs.migration-manager.finished-migrations /var/vcap/sys/log/bbs/bbs.stdout | wc -l
      ```

      The output from the command should larger than `1` on at least one
      database_z vm. Otherwise, that means `BBS` is still using `ETCD` and
      hasn't migrated the data over to the SQL backend

   2. Verify that data was migrated succesfully
      ``` shell
      bosh -d /path/to/diego.yml ssh database_z1/0 "source /var/vcap/jobs/cfdot/bin/setup && echo -n "number of desired lrps: " && cfdot desired-lrps | wc -l" 2>&1  | grep desired | grep -v Executing
      ```

      Or manually by sshing into `database_z1` and running the following commands:
      ```shell
      source /var/vcap/jobs/cfdot/bin/setup
      cfdot desired-lrps | wc -l
      ```

      You should see a number larger than 0, for example:

      ``` shell
      number of desired lrps:5
      ```

3. Configure Diego to connect only to a relational store, as with the `-x` flag to the manifest-generation script.
4. Deploy Diego and verify that the etcd jobs are no longer present in the deployment.
   Run the following command where `/path/to/diego.yml` is the path to the diego deployment manifest:

   ``` shell
   bosh -d /path/to/diego.yml ssh database_z1/0 "sudo /var/vcap/bosh/bin/monit summary" 2>&1 | grep etcd | wc -l
   ```

   Or manually by sshing into all `database_z` vms and running the following:
   ```shell
   sudo /var/vcap/bosh/bin/monit summary | grep etcd | wc -l
   ```

   this command should output `0`. A number larger than `0` means that `ETCD` is still running.

Support for migration from etcd to a relational datastore will be maintained through all 1.x versions of Diego.

### <a name="choosing-relational-datastore-deployment"></a>Choosing a Relational Data Store Deployment

Operators have a choice of deployment styles for both MySQL and PostgreSQL data stores. When Diego is deployed to accompany a CF deployment, operators will already have made a choice of database for the Cloud Controller and UAA databases, and it is expected that the same choice will be appropriate for the Diego data store.

#### MySQL

For MySQL, operators have at least the following options:

* Use the [CF-MySQL release](http://bosh.io/releases/github.com/cloudfoundry/cf-mysql-release?all=1) in standalone mode as a separate BOSH deployment, either as a single node, or as a highly available (HA) cluster.
* Use an infrastructure-specific database deployment, such as an RDS MySQL instance on AWS.

We recommend using at least version v27 of the CF-MySQL release.


#### PostgreSQL

For PostgreSQL, operators have at least the following options:

* Use the [PostgreSQL job](https://github.com/cloudfoundry/cf-release/tree/master/jobs/postgres) from the CF release, either sharing an existing instance that houses the CC and UAA databases, or deploying a separate node specifically for Diego.
* Use an infrastructure-specific database deployment, such as an RDS PostgreSQL instance on AWS.

**Note**: Diego requires a PostgreSQL version of 9.4 or higher for its data store.
