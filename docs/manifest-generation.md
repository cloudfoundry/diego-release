# Diego Manifest Generation

This document is for describing options to Diego manifest generation.

## diego-release/scripts/generate-deployment-manifest

#### SYNOPSIS:
    Generate a manifest for a Diego deployment to accompany an existing CF deployment.

#### USAGE:
    generate-deployment-manifest <MANDATORY ARGUMENTS> [OPTIONAL ARGUMENTS]

#### MANDATORY ARGUMENTS:
    -c <cf-path>        Path to CF manifest file.
    -i <iaas-path>      Path to IaaS-settings stub file.
    -p <property-path>  Path to property-overrides stub file.

#### OPTIONAL ARGUMENTS:
    -n <count-path>     Path to instance-count-overrides stub file.
    -v <versions-path>  Path to release-versions stub file.
    -s <sql-db-path>    Path to SQL stub file.
    -x                  Opt out of deploying etcd with the database vms (requires sql)
    -b                  Opt into using capi-release for bridge components.
    -d <voldriver-path> Path to voldriver stub file.
    -N <netman-path>    Path to netman stub file.
    -L                  Opt into using garden-linux-release for cells. (DEPRECATED)

#### EXAMPLE:
    scripts/generate-deployment-manifest \
      -c ../cf-release/bosh-lite/deployments/cf.yml \
      -i manifest-generation/bosh-lite-stubs/iaas-settings.yml \
      -p manifest-generation/bosh-lite-stubs/property-overrides.yml \
      -n manifest-generation/bosh-lite-stubs/instance-count-overrides.yml \
      -v manifest-generation/bosh-lite-stubs/release-versions.yml \
      -s manifest-generation/bosh-lite-stubs/mysql/diego-sql.yml \
      -x \
      -d manifest-generation/bosh-lite-stubs/experimental/voldriver/drivers.yml \
      -N manifest-generation/bosh-lite-stubs/experimental/netman/stub.yml \
      -b

### Options

#### -x Opt out of deploying etcd with the database VMs
When fully migrated data from an etcd release to SQL, or a fresh install using SQL use the -x flag to not deploy etcd to the database VMs.

#### -L Opt into using garden-linux-release for cells (DEPRECATED)
Use garden-linux-release instead of garden-runc as the container backend.

**Note**: garden-runc is the replacement for garden-linux-release, and we strongly recommend migrating to garden-runc.

#### -b Opt into using capi-release for bridge components
Use the cc-bridge components (e.g., stager, nsync, tps, etc.) from capi-release instead of cf-release.

### Stubs

#### -c Path to CF Manifest File
To specify the CF manifest and used to pull CF related properties into the generated Diego manifest

#### -i IaaS settings stub file
The  file to specify the IaaS specific values.  Items such as the subnet-configs, stemcell specifications etc.

##### bosh-lite example:
The bosh-lite IaaS-settings example can be found [iaas-settings.yml](https://github.com/cloudfoundry/diego-release/blob/develop/manifest-generation/bosh-lite-stubs/iaas-settings.yml).

#### -p Property overrides stub file
The  file to override specific diego properties

##### Bosh-lite example:
The bosh-lite property-overrides example can be found [property-overrides.yml](https://github.com/cloudfoundry/diego-release/blob/develop/manifest-generation/bosh-lite-stubs/property-overrides.yml)

#### -n Instance count overrides stub file (optional)
The file is used override the instance count for jobs in the diego manifest

##### bosh-lite example:
The bosh-lite instance-count-overrides example can be found [instance-count-overrides.yml](https://github.com/cloudfoundry/diego-release/blob/develop/manifest-generation/bosh-lite-stubs/instance-count-overrides.yml)

#### -v Release versions override stub file (optional)
The file is used to override the default (latest) release version for the releases used in the manifest

##### Example:
```yaml
release-versions:
  etcd: 35
  cflinuxfs2-rootfs: 1.12.0
  diego: 1.1450.0
  garden-runc: 1.0.2
```

##### -s SQL stub file

The optional -s flag is used to specify the stub for SQL and needs to be specific to either MySQL or Postgres.

##### MySQL Example:

```yaml
sql_overrides:
  bbs:
    db_driver: mysql
    db_host: 10.244.7.2
    db_port: 3306
    db_username: diego
    db_password: diego
    db_schema: diego
    max_open_connections: 500
```

##### Postgres Example:
```yaml
sql_overrides:
  bbs:
    db_driver: postgres
    db_host: 10.244.0.30
    db_port: 5524
    db_username: diego
    db_password: admin
    db_schema: diego
    max_open_connections: 500
```

##### **Experimental** -d Volume Stub File

The optional -d flag is used to specify the file for volume drivers.

##### Example:

```yaml
volman_overrides:
  releases:
  - name: cephfs-bosh-release
    version: "latest"
  driver_templates:
  - name: cephdriver
    release: cephfs-bosh-release
```

##### **Experimental** -N Container Networking Stub File

The optional -N flag is used to specify the path for the [netman](https://github.com/cloudfoundry-incubator/netman-release) stub file.
Netman requires Garden-runC, therefore the -g flag must also be set.

## diego-release/scripts/generate-windows-cell-deployment-manifest

#### SYNOPSIS:
    Generate a windows manifest for a Diego deployment to accompany an existing CF deployment.

#### USAGE:
    generate-windows-cell-deployment-manifest <MANDATORY ARGUMENTS> [OPTIONAL ARGUMENTS]

#### MANDATORY ARGUMENTS:
    -c <cf-path>        Path to CF manifest file.
    -i <iaas-path>      Path to IaaS-settings stub file.
    -p <property-path>  Path to property-overrides stub file.

#### OPTIONAL ARGUMENTS:
    -n <count-path>     Path to instance-count-overrides stub file.
    -v <versions-path>  Path to release-versions stub file.

#### EXAMPLE:
    generate-windows-cell-deployment-manifest \
      -c ../cf-release/bosh-lite/deployments/cf.yml \
      -i manifest-generation/bosh-lite-stubs/iaas-settings.yml \
      -p manifest-generation/bosh-lite-stubs/property-overrides.yml \
      -n manifest-generation/bosh-lite-stubs/instance-count-overrides.yml \
      -v manifest-generation/bosh-lite-stubs/release-versions.yml

### Stubs

#### -c Path to CF Manifest File
To specify the CF manifest and used to pull CF related properties into the generated Diego Windows manifest

#### -i IaaS settings stub file
The file to specify the IaaS specific values.  Items such as the subnet-configs, stemcell specifications etc.

#### -p Property overrides stub file
The file to override specific Diego Windows properties

#### -n Instance count overrides stub file (optional)
The file is used override the instance count for jobs in the Diego Windows manifest

#### -v Release versions override stub file (optional)
The file is used to override the default (latest) release version for the releases used in the manifest
