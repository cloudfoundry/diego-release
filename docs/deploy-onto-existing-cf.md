# Deploying Diego alongside an existing CF Deployment

This document describes the high-level overview for deploying a new Diego deployment into an environment with an existing CF deployment

### Table of Contents

1. [Setup additional IaaS requirements](#setup-additional-iaas-requirements)
1. [Generate CF Deployment with Diego requirements](#generate-cf-deployment-with-diego-requirements)
1. [Generate Diego deployment](#generate-diego-deployment)
1. [Upload additional releases](#upload-additional-releases)
1. [Deploy relational datastore (Optional)](#deploy-relational-datastore-optional)
1. [Create and Upload Diego Release](#create-and-upload-diego-release)
1. [Deploy Diego](#deploy-diego)

### Setup additional IaaS requirements

#### Diego Subnets

Create the required subnets in your IaaS for Diego.  If you are using multiple availability zones create one subnet per zone.

#### Load Balancer

Create a load balancer for the diego SSH proxy access.  There should be a single load balancer to distribute the load across all availability zones and access VMs in Diego.

### Generate CF Deployment with Diego requirements

The default deployment configuration from the manifest-generation scripts in cf-release omits some instances and properties that Diego depends on. It also includes some instances and properties that are unnecessary for a deployment with Diego as the only container runtime.

The changes required to an existing CF manifest include the following:

* Set all hm9000 instance counts to 0 (This may be done later after migration of all apps to Diego cells)
* Set all runner instance counts to 0 (This may be done later after migration of all apps to Diego cells)
* There must be at least 1 consul instance running.  Make sure you have a compatible version of CF for the targeted Diego release and set the consul VMs to running.
* The following properties should be included in the manifest:

```yaml
properties:
  app_ssh:
    host_key_fingerprint: (( merge ))
    oauth_client_id: ssh-proxy
  cc:
    default_to_diego_backend: true
    allow_app_ssh_access: true
  uaa:
    clients:
      <<: (( merge ))
      ssh-proxy:
        authorized-grant-types: authorization_code
        autoapprove: true
        override: true
        redirect-uri: /login
        scope: openid,cloud_controller.read,cloud_controller.write,cloud_controller.admin
        secret: ssh-prox y-secret
```
`default_to_diego_backend` and `allow_app_ssh_access` are optional and can be set later.

After these changes have been mode you will need to redeploy CF.

### Generate Diego Deployment

The Diego deployment relies heavily on the existing CF deployment and the generation of the manifest requires the `cf.yml`.   A sample script to run to generate the manifest is below:

```
cd $DIEGO_RELEASE_DIR
./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -n $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego/release-versions.yml \
  > $DEPLOYMENT_DIR/deployments/diego.yml
```

* The `cf.yml` specified is the manifest as updated in the previous step.
* You can find [examples](../examples/minimal-aws/stubs) of the other stubs required as input to the generation script.

### Upload additional releases

The Diego deployment requires several additional bosh releases to be uploaded to the bosh director as they are referenced in the Diego manifest.

The releases to upload can be found at http://bosh.io/releases and include the following:

* Container runtime
	* [garden-linux](http://bosh.io/releases/github.com/cloudfoundry-incubator/garden-linux-release?all=1)
	* [garden-runc](http://bosh.io/releases/github.com/cloudfoundry-incubator/garden-runc-release?all=1)
* [etcd-release](http://bosh.io/releases/github.com/cloudfoundry-incubator/etcd-release?all=1)
* [cflinuxfs2-rootfs](http://bosh.io/releases/github.com/cloudfoundry/cflinuxfs2-rootfs-release?all=1)

### Deploy relational datastore (Optional) 

See [documentation](datastores.md) on selecting a datastore and the options for deployment.

### Create and Upload Diego Release

The official Diego releases can be found [here](http://bosh.io/releases/github.com/cloudfoundry/diego-release?all=1)

Alternatively the [diego-release](http://github.com/cloudfoundry/diego-release) can be cloned and you can upload a development release to the bosh director using the following commands:

```bash
cd $DIEGO_RELEASE_DIR
bosh create release && bosh upload release
```

### Deploy Diego

Once all the dependencies have been uploaded to the bosh director you can deploy Diego to your environment using the following command:

```bash
bosh -d <path to diego manifest yml> deploy
```
