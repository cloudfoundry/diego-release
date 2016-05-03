# MySQL Backend for Diego

These instructions allow you to either:

* Provision an RDS MySQL Instance as a backend
* Provision a stand-alone CF-MySQL release
* Configure Diego to use one of the above configurations

## Table of Contents

1. [Setup RDS MySQL](#setup-aws-rds-mysql)
1. [Deploy Standalone CF-MySQL](#deploy-standalone-cf-mysql)
1. [Deploying Diego](#deploying-diego)

## Setup AWS RDS MySQL
Support for using a SQL database instead of etcd for the backing store of Diego is still in the experimental phase. The instructions below describe how to set up a MariaDB RDS instance that is known to work with Diego.

1. From the AWS console homepage, click on `RDS` in the `Database` section.
1. Click on `Launch DB Instance` under Instances.
1. Click on the `MariaDB` Tab and click the `Select` button.
1. Select Production or Dev/Test version of MariaDB depending on your use case and click the `Next Step` button.
1. Select the DB Instance Class required. For performance testing the Diego team uses db.m4.4xlarge.
1. Optionally tune the other parameters based on your deployment requirements.
1. Provide a unique DB Instance Identifier.
1. Choose and confirm a master username and password, and record them for later use in the diego-sql stub.
1. Click `Next Step`.
1. Select the VPC created during the bosh-init steps above.
1. Select `No` for the `Publicly Accessible` option.
1. Select the `VPC Security Group` matching `*-InternalSecurityGroup-*`.
1. Choose a Database Name (for example, `diego`).
1. Click `Launch DB Instance`.
1. Wait for the Instance to be `available`.

## Deploy Standalone CF-MySQL
Follow the instructions at [CF MySQL Deploy](https://github.com/cloudfoundry/cf-mysql-release#deploy-on-aws-or-vsphere) to deploy a stand alone example [examples/standalone](https://github.com/cloudfoundry/cf-mysql-release/blob/develop/manifest-generation/examples/standalone)

To minimize the deployment to only a single MySQL node use the following instance-count-overrides.yml

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

## Deploy Diego

### Fill in diego-sql Stub (optional)

You will also need a stub with the SQL instance details.  Create a file `$DEPLOYMENT_DIR/stubs/diego/diego-sql.yml` with the following contents:

```yaml
sql_overrides:
  bbs:
    db_connection_string: '<username>:<password>@tcp(<sql-instance-endpoint>)/<database-name>'
    max_open_connections: 500
```

Fill in the bracketed parameters above with the following values:

- `<username>`: The username chosen when you created the SQL instance.
- `<password>`: The password chosen when you created the SQL instance.
- `<sql-instance-endpoint>`: 
	- For AWS RDS - The endpoint displayed at the top of the DB instance details page in AWS, including the port.
	- For CF-MySQL - The IP and Port of the SQL Node (e.g. 10.10.5.222:3306)
- `<database-name>`: the name chosen when you created the SQL instance.

### Generate the Diego manifest

Remember that the last two arguments for `instance-count-overrides` and `release-versions`
are optional.

```bash
cd $DIEGO_RELEASE_DIR
./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -n $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego/release-versions.yml \
  -s $DEPLOYMENT_DIR/stubs/diego/diego-sql.yml \
  > $DEPLOYMENT_DIR/deployments/diego.yml
```