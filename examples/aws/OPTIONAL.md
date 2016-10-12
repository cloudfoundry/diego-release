# Optional Configurations for Diego

## Table of Contents

1. [Setup a SQL database for Diego](#setup-a-sql-database-for-diego)
  * [Setup RDS MySQL](#setup-aws-rds-mysql) *OR*
  * [Deploy Standalone CF-MySQL](#deploy-standalone-cf-mysql)
  * [Deploying Diego](#deploy-diego)
1. [Setup Volume Drivers for Diego](#setup-volume-drivers-for-diego)
1. [Setup Garden RunC for Diego](#setup-garden-runc-for-diego)
1. [Setup Garden Windows for Diego](#setup-garden-windows-for-diego)

## Setup a SQL database for Diego

These instructions allow you to either:

* Provision an RDS MySQL Instance as a backend
* Provision a stand-alone CF-MySQL release
* Configure Diego to use one of the above configurations

We support two ways of providing a SQL database. They are:

* [Setup RDS MySQL](#setup-aws-rds-mysql) *OR*
* [Deploy Standalone CF-MySQL](#deploy-standalone-cf-mysql)

### Setup AWS RDS MySQL

The instructions below describe how to set up a *MariaDB* RDS instance that is
known to work with Diego.

1. From the AWS console homepage, click on `RDS` in the `Database` section.
1. Click on `Launch DB Instance` under Instances.
1. Click on the `MariaDB` Tab and click the `Select` button.
1. Select Production or Dev/Test version of MariaDB depending on your use case and click the `Next Step` button.
1. Select the DB Instance Class required. For performance testing the Diego team uses db.m4.4xlarge.
1. Optionally tune the other parameters based on your deployment requirements.
1. Provide a unique DB Instance Identifier. This identifier can be arbitrary, as is not used directly in the Diego configuration below.
1. Choose and confirm a master username and password, and record them for later use in the Diego-SQL stub.
1. Click `Next Step`.
1. Select the VPC created during the bosh-init steps above.
1. Select `No` for the `Publicly Accessible` option.
1. Select the `VPC Security Group` matching `*-InternalSecurityGroup-*`.
1. Choose a Database Name (for example, `diego`).
1. Click `Launch DB Instance`.
1. Wait for the Instance to be `available`.

#### Configuring SSL

In order to configure SSL for RDS you need to download the ca cert bundle from AWS. This can be done by:

```bash
curl -o $DEPLOYMENT_DIR/certs/rds-combined-ca-bundle.pem http://s3.amazonaws.com/rds-downloads/rds-combined-ca-bundle.pem
```

The contents of this file will be supplied in the `sql_overrides.bbs.ca_cert` field in the Diego-SQL stub below.

### Deploy Standalone CF-MySQL

The CF-MySQL release can be deployed in a few different modes and
configurations. All configurations have the same starting steps:

1. Make a directory for the CF-MySQL stubs:

  ```bash
  mkdir -p $DEPLOYMENT_DIR/stubs/cf-mysql
  ```

1. Clone the CF-MySQL release `release-candidate` branch:

  ```bash
  git clone -b release-candidate https://github.com/cloudfoundry/cf-mysql-release.git
  export CF_MYSQL_RELEASE_DIR=$PWD/cf-mysql-release
  ```

1. Copy over relevant stubs from the CF-MySQL release to deployment directory:

  ```bash
  cp $CF_MYSQL_RELEASE_DIR/manifest-generation/examples/standalone/property-overrides.yml \
     $CF_MYSQL_RELEASE_DIR/manifest-generation/examples/standalone/instance-count-overrides.yml \
  $DEPLOYMENT_DIR/stubs/cf-mysql/
  ```

1. Copy over CF-based manifest stub:

  ```bash
  cp $DEPLOYMENT_DIR/deployments/cf.yml $DEPLOYMENT_DIR/stubs/cf-mysql/cf.yml
  ```

1. Edit `property-overrides.yml`:
  1. Rename the deployment:

    ```yaml
    property_overrides:
      deployment_name: diego-mysql
    ```

  1. Fill in all `REPLACE_WITH_` properties with appropriate values. Ignore all `UNUSED_VALUE` properties.
  1. Set the `host` property to `null`. Do not remove it entirely, since the
     current manifest-generation scripts for CF-MySQL depend on its presence:

    ```yaml
    property_overrides:
      host: null
    ```

  1. Add the following `seeded_databases` property to configure a database for Diego to use. Replace `REPLACE_ME_WITH_DB_PASSWORD` with the desired password for the database:

    ```yaml
    property_overrides:
      mysql:
        seeded_databases:
        - name: diego
          username: diego
          password: REPLACE_ME_WITH_DB_PASSWORD
    ```

After that you can deploy the CF-MySQL release in either mode:

* [Single Node CF-MySQL](#single-node-cf-mysql) - Used mostly for development
  and experimentation since it does not provide the same uptime guarantees that
  the multi-node deployment does.
* [Highly Available CF-MySQL](#highly-available-cf-mysql) - Recommended for
  production use. Uses [Consul](https://consul.io) for discovery.

#### Single Node CF-MySQL

1. Edits to `instance-count-overrides.yml`:

  ```yaml
  instance_count_overrides:
    - name: cf-mysql-broker_z1
      instances: 0
    - name: cf-mysql-broker_z2
      instances: 0
    - name: mysql_z2
      instances: 0
    - name: arbitrator_z3
      instances: 0
    - name: proxy_z1
      instances: 0
    - name: proxy_z2
      instances: 0
  ```

1. Generate deployment manifest:

  ```bash
  $CF_MYSQL_RELEASE_DIR/scripts/generate-deployment-manifest \
      -c $DEPLOYMENT_DIR/stubs/cf-mysql/cf.yml \
      -p $DEPLOYMENT_DIR/stubs/cf-mysql/property-overrides.yml \
      -i $DEPLOYMENT_DIR/stubs/cf-mysql/iaas-settings.yml \
      -n $DEPLOYMENT_DIR/stubs/cf-mysql/instance-count-overrides.yml \
  > $DEPLOYMENT_DIR/deployments/cf-mysql.yml
  ```

1. Deploy the CF-MySQL cluster

  ```bash
  cd $CF_MYSQL_RELEASE_DIR
  bosh create release && bosh upload release && bosh -d $DEPLOYMENT_DIR/deployments/cf-mysql.yml deploy
  ```

#### Highly Available CF-MySQL

1. Copy additional `job-overrides-consul.yml`:

  ```bash
  cp $CF_MYSQL_RELEASE_DIR/manifest-generation/examples/job-overrides-consul.yml \
  $DEPLOYMENT_DIR/stubs/cf-mysql/
  ```

1. Edits to `property-overrides.yml`, add the following properties:

  ```yaml
  property_overrides:
    proxy:
      # ...
      consul_enabled: true
      consul_service_name: mysql
  ```

1. Generate deployment manifest:

  ```bash
  $CF_MYSQL_RELEASE_DIR/scripts/generate-deployment-manifest \
      -c $DEPLOYMENT_DIR/stubs/cf-mysql/cf.yml \
      -p $DEPLOYMENT_DIR/stubs/cf-mysql/property-overrides.yml \
      -i $DEPLOYMENT_DIR/stubs/cf-mysql/iaas-settings.yml \
      -j $DEPLOYMENT_DIR/stubs/cf-mysql/job-overrides-consul.yml \
      -n $DEPLOYMENT_DIR/stubs/cf-mysql/instance-count-overrides.yml \
  > $DEPLOYMENT_DIR/deployments/cf-mysql.yml
  ```

1. Deploy the CF-MySQL cluster

  ```bash
  bosh -d $DEPLOYMENT_DIR/deployments/cf-mysql.yml deploy
  ```

## Deploy Diego

### Fill in Diego-SQL stub

To configure Diego to communicate with the SQL instance, first create a Diego-SQL stub file at `$DEPLOYMENT_DIR/stubs/diego/diego-sql.yml` with the following contents:

```yaml
sql_overrides:
  bbs:
    db_connection_string: 'diego:REPLACE_ME_WITH_DB_PASSWORD@tcp(<sql-instance-endpoint>)/diego'
    max_open_connections: 500
    require_ssl: null
    ca_cert: null
```

Fill in the bracketed parameters in the `db_connection_string` with the following values:

- `REPLACE_ME_WITH_DB_PASSWORD`: The password chosen when you created the SQL instance.
- `<sql-instance-endpoint>`:
  - For AWS RDS: The endpoint displayed at the top of the DB instance details page in AWS, including the port.
  - For Standalone CF-MySQL:
    - If configuring a Single Node CF-MySQL the internal IP address and port of the single MySQL node. (e.g. `10.10.5.222:3306`).
    - If configuring an Highly Available CF-MySQL with Consul use the consul service address (e.g. `mysql.service.cf.internal:3306`).
    - *In both cases the port will be `3306` by default.*

**Note:** The `sql_overrides.bbs.ca_cert` and `sql_overrides.bbs.require_ssl` properties should be provided only when deploying with an SSL-supported MySQL cluster. Set the `require_ssl` property to `true` to ensure that the BBS uses SSL to connect to the store, and set the `ca_cert` property to the contents of a certificate bundle containing the correct CA certificates to verify the certificate that the SQL server presents.

If enabling SSL for an RDS database, include the contents of `$DEPLOYMENT_DIR/certs/rds-combined-ca-bundle.pem` as the value of the `ca_cert` property:

```yaml
sql_overrides:
  bbs:
    ca_cert: |
      REPLACE_WITH_CONTENTS_OF_(DEPLOYMENT_DIR/certs/rds-combined-ca-bundle.pem)
```

### Generate the Diego manifest

Generate the Diego manifest with an additional `-s` flag that specifies the location of the Diego-SQL stub, as shown below. Remember that the `-n` instance-count-overrides flag and the `-v` release-versions flags are optional.

```bash
cd $DIEGO_RELEASE_DIR
./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -s $DEPLOYMENT_DIR/stubs/diego/diego-sql.yml \
  -n $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego/release-versions.yml \
  > $DEPLOYMENT_DIR/deployments/diego.yml
```

### Disable and remove ETCD from your Diego deployment

Once you've successfully deployed a SQL-backed Diego, you may want to remove the now-idle etcd jobs from your database cluster to save on infrastructure costs. Once the database VMs are free of etcd jobs, they do not need to be deployed with write-optimized disks.

To remove etcd from your deployment, invoke the manifest-generation script with the `-x` flag:

```bash
cd $DIEGO_RELEASE_DIR
./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -s $DEPLOYMENT_DIR/stubs/diego/diego-sql.yml \
  -x \
  -n $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego/release-versions.yml \
  > $DEPLOYMENT_DIR/deployments/diego.yml
```

## Setup Volume Drivers for Diego

To co-locate a driver on the Diego cells, first create a Drivers stub file at `$DEPLOYMENT_DIR/stubs/diego/drivers.yml` with the following contents:

```yaml
volman_overrides:
  releases:
  - name: REPLACE_WITH_DRIVER_BOSH_RELEASE
    version: REPLACE_WITH_DRIVER_BOSH_RELEASE_VERSION
  driver_templates:
  - name: REPLACE_WITH_DRIVER_TEMPLATE
    release: REPLACE_WITH_DRIVER_BOSH_RELEASE
```

Replace all `REPLACE_WITH_DRIVER_*` entries with the values of your driver's bosh release.

If you wish to use the `cephdriver` that we use for testing and development you may use the following stub:-

```yaml
volman_overrides:
  releases:
  - name: cephfs-bosh-release
    version: "latest"
  driver_templates:
  - name: cephdriver
    release: cephfs-bosh-release
```

Now return to the previous steps to [generate the Diego manifest](README.md#generate-the-diego-manifest) and supply the `-d ` flag that specifies the location of this Drivers stub file, as the example below demonstrates.

```
cd $DIEGO_RELEASE_DIR
  ./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings-internal.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -n $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego/release-versions.yml \
  -d $DIEGO_RELEASE_DIR/stubs/diego/drivers.yml \
  > $DEPLOYMENT_DIR/deployments/diego.yml
```

Diego volume services use Docker Volume Plugins to manage volume mounts on each of the Cells.  These drivers are discovered by looking in a specific location for Docker Volume Plugin configuration files (for more information on Docker plugin configuration files see `Plugin discovery` in the Docker [documentation](https://docs.docker.com/engine/extend/plugin_api/)).  This location defaults to `/var/vcap/data/voldrivers` however it may be overriden by specifying the `diego.executor.volman.driver_paths` property in one or more of the Cell job templates in your generated diego manifest as the following example shows:

```
- instances: 1
  name: cell_z1
  networks:
  - name: diego1
  properties:
    diego:
      rep:
        zone: z1
      executor:
        volman:
          driver_paths: /etc/docker/plugins
    metron_agent:
      zone: z1
    ...
```

## Setup Garden RunC for Diego

Generate the Diego manifest with an additional `-g` flag that specifies opting into the Garden-RunC implementation on the Diego Cells.

```bash
cd $DIEGO_RELEASE_DIR
./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -s $DEPLOYMENT_DIR/stubs/diego/diego-sql.yml \
  -n $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego/release-versions.yml \
  -g
  > $DEPLOYMENT_DIR/deployments/diego.yml
```

When deploying Garden-RunC on a previously deployed Diego with Garden-linux you must recreate the cells as upgrade is not supported by Garden-RunC.

```bash
bosh -n deploy --recreate
```

## Setup Garden Windows for Diego

### Upload Windows Stemcell

You will need to download the windows bosh stemcell and upload it to the bosh director with the following:

```
wget https://s3.amazonaws.com/bosh-windows-stemcells/light-bosh-stemcell-0.0.50-aws-xen-hvm-windows2012R2-go_agent.tgz
bosh upload stemcell light-bosh-stemcell-0.0.50-aws-xen-hvm-windows2012R2-go_agent.tgz
```

### Upload Garden Windows Bosh Release

In order to successfully deploy Diego Windows cells, you will need to upload the following bosh release:

```
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-windows-release
```

### Generate Diego Windows Cell Deployment Manifest

See the full [manifest generation documentation](https://github.com/cloudfoundry/diego-release/docs/manifest-generation.md) for more generation instructions.
Remember that the `-n` instance-count-overrides flag and the `-v` release-versions flags are optional.

```bash
cd $DIEGO_RELEASE_DIR
./scripts/generate-windows-cell-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego-windows/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -n $DEPLOYMENT_DIR/stubs/diego-windows/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego-windows/release-versions.yml \
  > $DEPLOYMENT_DIR/deployments/diego-windows.yml
```

### Deploy

```bash
bosh deployment $DEPLOYMENT_DIR/deployments/diego-windows.yml
bosh deploy
```
