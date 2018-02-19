# Data Stores for Diego

This document describes the different types of data store supported by Diego across its different versions.

### Table of Contents

1. [Supported Data Stores](#supported-data-stores)
1. [Choosing a Relational Data Store Deployment](#choosing-relational-datastore-deployment)


### <a name="supported-data-stores"></a>Supported Data Stores

Diego v1.0 and later support only a relational database as the data store for the BBS API server. Both the MySQL and PostgreSQL dialects of SQL are supported on Diego v0.1480.0 and later.


### <a name="choosing-relational-datastore-deployment"></a>Choosing a Relational Data Store Deployment

Operators have a choice of deployment styles for both MySQL and PostgreSQL data stores. When Diego is deployed to accompany a CF deployment, operators will already have made a choice of database for the Cloud Controller and UAA databases, and it is expected that the same choice will be appropriate for the Diego data store.

#### MySQL

For MySQL, operators have at least the following options:

* Use the [CF-MySQL release](http://bosh.io/releases/github.com/cloudfoundry/cf-mysql-release?all=1) in standalone mode as a separate BOSH deployment, either as a single node, or as a highly available (HA) cluster.
* Use an infrastructure-specific database deployment, such as an RDS MySQL instance on AWS.

We recommend using at least version v36 of the CF-MySQL release.

**Note**: Diego requires a MariaDB version of 10.1.24 or higher for its data store.

#### PostgreSQL

For PostgreSQL, operators have at least the following options:

* Use the [Postgres release](https://bosh.io/releases/github.com/cloudfoundry/postgres-release?all=1) in standalone mode as a separate BOSH deployment, either as a single node, or as a highly available (HA) cluster.
* Use an infrastructure-specific database deployment, such as an RDS PostgreSQL instance on AWS.

We recommend using at least version v25 of the Postgres release.

**Note**: Diego requires a PostgreSQL version of 9.6.6 or higher for its data store.


### <a name="automatic-migration-bbs-data-etcd-sql"></a>Automatic Migration of BBS Data from etcd to SQL

As of Diego v2.0, support for automatic data migration from etcd to sql has been removed.
