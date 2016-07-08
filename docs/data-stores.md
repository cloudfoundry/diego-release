# Data Stores for Diego

This document describes the different types of data store supported by Diego across its different versions.

### Table of Contents

1. [Supported Data Stores](#supported-data-stores)
1. [Choosing a Relational Data Store Deployment](#choosing-relational-datastore-deployment)


### <a name="supported-data-stores"></a>Supported Data Stores

At present, Diego supports the following types of data store to back the BBS API server:

* etcd key-value store
* Relational database: MySQL or PostgreSQL

#### etcd

Diego has used etcd as its backing data store since its initial versions, and will continue to do so through every release on major version 0. Support for etcd as the primary backing store will end in major version 1, though, as Diego changes to support relational data stores to back the BBS server.


#### Relational Databases

Diego supports two SQL dialects, MySQL and PostgreSQL, when configured to use a relational data store to back the BBS. Official support for them will start with a forthcoming Diego release version.

#### Migration of BBS Data from etcd to SQL

For a Diego deployment that has previously been configured to use etcd as its data store, configuring it also to connect to a relational store will cause the BBS to migrate the etcd data to the relational store automatically. After the migration, the data in etcd is marked as invalid, and Diego will not revert to using its data without manual intervention in the etcd key-value store to reset the version.

Converting Diego from standalone etcd to standalone relational requires two deploys:

1. Configure Diego to connect both to etcd and to the relational store, as with the `-s` flag to the [manifest-generation script](./manifest-generation.md).
1. Deploy Diego and verify that the BBS nodes have migrated the etcd data to the relational store.
1. Configure Diego to connect only to a relational store, as with the `-x` flag to the manifest-generation script.
1. Deploy Diego and verify that the etcd jobs are no longer present in the deployment.

Support for migration from etcd to a relational datastore will be maintained through all 1.x versions of Diego.

### <a name="choosing-relational-datastore-deployment"></a>Choosing a Relational Data Store Deployment

Operators have a choice of deployment styles for both MySQL and PostgreSQL data stores. When Diego is deployed to accompany a CF deployment, operators will already have made a choice of database for the Cloud Controller and UAA databases, and it is expected that the same choice will be appropiate for the Diego data store.

#### MySQL

For MySQL, operators have at least the following options:

* Use the [CF-MySQL release](http://bosh.io/releases/github.com/cloudfoundry/cf-mysql-release?all=1) in standalone mode as a separate BOSH deployment, either as a single node, or as a highly available (HA) cluster.
* Use an infrastructure-specific database deployment, such as an RDS MySQL instance on AWS.

At present, the latest final CF-MySQL release, v26, will be acceptable in an HA configuration to back Diego. The next major version, v27, will contain additional improvements for operating an HA deployment to support a large-scale Diego deployment. These features are already available on the release-candidate branch of the cf-mysql-release repository.

#### PostgreSQL

For PostgreSQL, operators have at least the following options:

* Use the [PostgreSQL job](https://github.com/cloudfoundry/cf-release/tree/master/jobs/postgres) from the CF release, either sharing an existing instance that houses the CC and UAA databases, or deploying a separate node specifically for Diego.
* Use an infrastructure-specific database deployment, such as an RDS PostgresQL instance on AWS.
