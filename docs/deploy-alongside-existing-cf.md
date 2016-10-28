# Deploying Diego Alongside an Existing CF Deployment

This document describes a high-level overview for deploying a new Diego deployment to integrate with an existing CF deployment.

### Table of Contents

1. [Configure Additional Infrastructure](#configure-additional-infrastructure)
1. [Configure CF Manifest for Diego](#configure-cf-manifest-for-diego)
1. [Generate Diego Deployment Manifest](#generate-diego-deployment-manifest)
1. [Upload Additional Releases](#upload-additional-releases)
1. [Deploy Relational Datastore (Optional)](#deploy-relational-datastore-optional)
1. [Create and Upload Diego Release](#create-and-upload-diego-release)
1. [Deploy Diego](#deploy-diego)

### <a name="configure-additional-infrastructure"></a>Configure Additional Infrastructure

#### Diego Subnets

Create the required subnets in your IaaS for Diego.  If you are using multiple availability zones, create one subnet per zone.

#### Load Balancer

If not using the [HAProxy job](https://github.com/cloudfoundry/cf-release/tree/master/jobs/haproxy) in the CF release for load balancing, create a load balancer for the Diego SSH proxy instances.

### <a name="configure-cf-manifest-for-diego"></a>Configure CF Manifest for Diego

The default deployment configuration from the manifest-generation scripts in cf-release omits some instances and properties that Diego depends on. It also includes some instances and properties that are unnecessary for a deployment with Diego as the only container runtime.

See the [cf/diego.yml stub](../examples/aws/stubs/cf/diego.yml) from the AWS documentation example for one example of these configuration changes.

#### Required Changes

* Change the instance counts on the `consul_z1` and `consul_z2` jobs so that at least one instance is running. At least three instances total are recommended for a highly available deployment.

#### Optional Changes

* If you no longer wish to deploy DEAs at all any more:
  * Set the `hm9000_z1` and `hm9000_z2` instance counts to 0.
  * Set the `runner_z1` and `runner_z2` instance counts to 0.
* For SSH access to instances, configure the `app_ssh` property section and add an appropriately configured `ssh-proxy` client to the list of UAA clients.
* To make Diego the default container runtime for CF, enable the `cc.default_to_diego_backend` property.


After these changes have been made, redeploy CF.


### <a name="generate-diego-deployment-manifest"></a>Generate Diego Deployment Manifest

The Diego deployment integrates with services in the CF deployment, and so the generation of the manifest requires data in the CF deployment manifest. The `generate-deployment-manifest` script takes that CF manifest as the input to its `-c` flag. For example:

```
cd $DIEGO_RELEASE_DIR
./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  > $DEPLOYMENT_DIR/deployments/diego.yml
```

The `cf.yml` file specified is the CF manifest as updated above.

See the [manifest-generation script documentation](manifest-generation.md) for more information about the script arguments. For examples of these input stubs, see the [BOSH-Lite deployment stubs](../manifest-generation/bosh-lite-stubs), [minimal AWS stubs](../examples/minimal-aws/stubs), or [full AWS documentation](../examples/aws).


### <a name="upload-additional-releases"></a>Upload Additional Releases

The Diego deployment requires several additional BOSH releases to be uploaded to the BOSH director, as they are referenced in the Diego manifest.

The releases to upload can be found at [bosh.io/releases](https://bosh.io/releases) and include the following:

* Container runtime: [garden-runc](http://bosh.io/releases/github.com/cloudfoundry-incubator/garden-runc-release?all=1)
* [cflinuxfs2-rootfs](http://bosh.io/releases/github.com/cloudfoundry/cflinuxfs2-rootfs-release?all=1)

If using etcd as the BBS data store instead of a relational data store, also upload [etcd-release](http://bosh.io/releases/github.com/cloudfoundry-incubator/etcd-release?all=1).

### <a name="deploy-relational-datastore-optional"></a>Deploy Relational Datastore (Optional)

See documentation on [data stores](data-stores.md) to select and deploy a relational data store instead of etcd to back the BBS server.

### <a name="create-and-upload-diego-release"></a>Create and Upload Diego Release

Official release tarballs for Diego can be found on [bosh.io](http://bosh.io/releases/github.com/cloudfoundry/diego-release?all=1).

Alternatively, clone the [diego-release repository](http://github.com/cloudfoundry/diego-release) and create and upload a development release to the BOSH director using the following commands:

```bash
cd $DIEGO_RELEASE_DIR
bosh create release && bosh upload release
```

### <a name="deploy-diego"></a>Deploy Diego

Once all the dependencies have been uploaded to the bosh director, deploy Diego to your environment:

```bash
bosh -d <path-to-diego-manifest-yml> deploy
```
