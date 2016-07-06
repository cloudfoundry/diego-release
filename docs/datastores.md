# Diego DataStores

This document describes the different types of datastore used by Diego and the version of Diego that supports each of the datastores.

### Table of Contents

1. [Supported DataStores](#supported-datastores)
1. [Choosing Relational DataStore Deployments](#choosing-relational-datastore-deployments)


### Supported DataStores

At the time of this writing, Diego supports three different types of datastore, namely:

* ETCD
* MySQL
* PostgreSQL

#### ETCD

Standalone version of ETCD was the first backing datastore for the BBS component of Diego. However, as Diego reaches the `v1.0` milestone, it has moved away from a non-relational datastore to use a relational datastore for its BBS component. As a result, the use of ETCD is being deprecated.

Standalone ETCD will be supported throughout all `0.x` versions, up until - but not including - version `1.0`.

#### Relational Databases

For the relational databases, Diego supports both MySQL and PostgreSQL to be used as the backing datastore for BBS. The relational support for Diego will be supported from version `0.y` forward, `y` to be defined soon.

If BBS is configured to connect to a relational database, in the event of having an existing ETCD datastore, it will automatically migrate the data in ETCD to the relational datastore. After the migration, the data in ETCD is no longer usable, and the migration is only a one-way migration, i.e., there is no possibility to switch back to an ETCD datastore after migrating to a relational datastore.

Support for migration from and ETCD datastore to a relational datastore will be maintained through all 1.x versions of Diego.

### Choosing Relational DataStore Deployments

#### MySQL

For MySQL, any of the following options are available:

* Using the standalone CF MySQL release as a separate [bosh deployment](http://bosh.io/releases/github.com/cloudfoundry/cf-mysql-release?all=1) either as a single node, or as a highly available (HA) cluster.
* Infrastructure-specific database deployments, such as AWS RDS MySQL

#### PostgreSQL

For PostgreSQL any of the following options are available:

* Using the PostgreSQL database from the CF Release
* Infrastructure-specific database deployments, such as AWS RDS PostgreSQL

