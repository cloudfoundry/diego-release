# Diego Manifest Generation

This document is for describing options to Diego manifest generation.

### Table of Contents

1. [Scripts](#scripts)
1. [Options](#options)
1. [Stubs](#stubs)

### Scripts

###1) diego-release/scripts/generate-deployment-manifest

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
    -g                  Opt into using garden-runc-release for cells.
    -b                  Opt into using capi-release for bridge components.
    -d <voldriver-path> Path to voldriver stub file.

#### EXAMPLE:
    scripts/generate-deployment-manifest \\
      -c ../cf-release/bosh-lite/deployments/cf.yml \\
      -i manifest-generation/bosh-lite-stubs/iaas-settings.yml \\
      -p manifest-generation/bosh-lite-stubs/property-overrides.yml \\
      -n manifest-generation/bosh-lite-stubs/instance-count-overrides.yml \\
      -v manifest-generation/bosh-lite-stubs/release-versions.yml \\
      -s manifest-generation/bosh-lite-stubs/diego-sql.yml \\
      -x \\
      -d manifest-generation/bosh-lite-stubs/experimental/voldriver/drivers.yml \\
      -g \\
      -b

### Options

#### -c Path to CF Manifest File
To specify the CF manifest and used to pull CF related properties into the generated Diego manifest

#### -x Opt out of deploying etcd with the database VMs
When fully migrated data from an etcd release to SQL, or a fresh install using SQL use the -x flag to not deploy etcd to the database VMs.

#### -g Opt into using garden-runc-release for cells
To use garden-runc release instead of garden-linux.

**Note**: Migration from garden-linux based cells to garden-runc cells is not supported.  Cells must be recreated if previously deployed using garden-linux.

#### -b Opt into using capi-release for bridge components
Use the cc-bridge components (e.g., stager, nsync, tps, etc.) from capi-release instead of cf-release.

### Stubs

#### IaaS settings stub file
The  file to specify the IaaS specific values.  Items such as the subnet-configs, stemcell specifications etc.

##### bosh-lite example:
The bosh-lite IaaS-settings example can be found [iaas-settings.yml](https://github.com/cloudfoundry/diego-release/blob/develop/manifest-generation/bosh-lite-stubs/iaas-settings.yml).

#### Property overrides stub file
The  file to override specific diego properties

##### Bosh-lite example:
The bosh-lite property-overrides example can be found [property-overrides.yml](https://github.com/cloudfoundry/diego-release/blob/develop/manifest-generation/bosh-lite-stubs/property-overrides.yml)

#### Instance count overrides stub file (optional)
The file is used override the instance count for jobs in the diego manifest

##### bosh-lite example:
The bosh-lite instance-count-overrides example can be found [instance-count-overrides.yml](https://github.com/cloudfoundry/diego-release/blob/develop/manifest-generation/bosh-lite-stubs/instance-count-overrides.yml)

#### Release versions override stub file (optional)
The file is used to override the default (latest) release version for the releases used in the manifest

##### Example:
```yaml
release-versions:
  etcd: 35
  cflinuxfs2-rootfs: 1.12.0
  garden-linux: 0.336.0
  diego: 1.1450.0
  garden-runc: 0.2.0
```

##### **Experimental** SQL stub file

The optional -s flag is used to specify the stub for SQL and needs to be specific to either MySQL or Postgres.

##### MySQL Example:

```yaml
sql_overrides:
  bbs:
    db_connection_string: 'diego:diego@tcp(10.244.7.2:3306)/diego'
    db_driver: mysql
    max_open_connections: 500
```

##### Postgres Example:
```yaml
sql_overrides:
  bbs:
    db_connection_string: 'postgres://diego:admin@10.244.0.30:5524/diego'
    db_driver: postgres
    max_open_connections: 500
```

##### **Experimental** Volume Stub File

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
