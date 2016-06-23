# Deploying a Minimal Diego to AWS

## Introduction

These instructions will allow you to deploy a minimal Diego that works with
the [minimal CF installation instructions](https://github.com/cloudfoundry/cf-release/tree/master/example_manifests).
The rest of these instructions will assume that you have followed the instructions
from [cf-release](https://github.com/cloudfounry/cf-release) to set up a minimal
CF deployment.

*NOTE*: This deployment is meant for instructional purposes only.
It should not be used in a production setting as it does not guarantee high
availability and lacks basic security configuration.

## Setup

### Create a Diego Subnet

- Click on "Subnets" from the VPC Dashboard
- Click "Create Subnet"
- Fill in
  - Name tag: diego
  - VPC: bosh
  - Availability Zone: Pick the same Availability Zone as the bosh Subnet
    - Replace REPLACE_WITH_DIEGO_SUBNET_AZ in the example manifest with the Availability Zone you chose
  - CIDR block: 10.0.18.0/24
  - Click "Yes, Create"
- Replace REPLACE_WITH_DIEGO_SUBNET_ID in the example manifest with the Subnet ID for the diego Subnet
- Select the `diego` Subnet from the Subnet list
- Click the name of the "Route table:" to view the route tables
- Select the route table from the list
- Click "Routes" in the bottom window
- Click "Edit"
- Fill in a new route
  - Destination: 0.0.0.0/0
  - Target: Select the NAT instance from the list
- Click "Save"

### Generate SSH Proxy Host Key and Fingerprint

```bash
ssh-keygen -f ssh-proxy-host-key.pem
```
If the local `ssh-keygen` supports the `-E` flag, as it does on OS X 10.11 El Capitan or Ubuntu 16.04 Xenial Xerus, generate the MD5 fingerprint of the public host key as follows:
```bash
ssh-keygen -lf ssh-proxy-host-key.pem.pub -E md5 | cut -d ' ' -f2 | sed 's/MD5://g' > ssh-proxy-host-key-fingerprint
```
Otherwise, generate the MD5 fingerprint as follows:
```bash
ssh-keygen -lf ssh-proxy-host-key.pem.pub | cut -d ' ' -f2 > ssh-proxy-host-key-fingerprint
```
The `ssh-proxy-host-key.pem` file contains the PEM-encoded private host key for the Diego manifest, and the `ssh-proxy-host-key-fingerprint` file contains the MD5 fingerprint of the public host key. You will later copy these values into stubs for generating the CF and Diego manifests.

Replace REPLACE_WITH_SSH_HOST_KEY in the example diego manifest with the contents of `ssh-proxy-host-key.pem`.

### Modify the CF Manifest with Diego Properties

Add the following properties to the [minimal-aws.yml](https://github.com/cloudfoundry/cf-release/blob/master/example_manifests/minimal-aws.yml):

```
properties:
  app_ssh:
    host_key_fingerprint: REPLACE_WITH_SSH_HOST_KEY_FINGERPRINT
    oauth_client_id: ssh-proxy
  cc:
    allow_app_ssh_access: true
    default_to_diego_backend: true
    internal_api_user: internal_user
  uaa:
    clients:
      cf:
      - ssh-proxy:
        authorized-grant-types: authorization_code
        autoapprove: true
        override: true
        redirect-uri: /login
        scope: openid,cloud_controller.read,cloud_controller.write,cloud_controller.admin
        secret: PASSWORD
```

Replace REPLACE_WITH_SSH_HOST_KEY_FINGERPRINT with the contents of `ssh-proxy-host-key-fingerprint`.

In order to properly SSH into application instances, you will also need to add the
`consul_agent` template to the `ha_proxy_z1` job definition as follows:

```
- name: ha_proxy_z1
  instances: 1
  resource_pool: small_z1
  templates:
  - {name: haproxy, release: cf}
  - {name: consul_agent, release: cf}
  - {name: metron_agent, release: cf}
```

*Note*: If you no longer want to be able to run applications on the DEAs, you can decrease the
number of instances for the `runner_z1` and `hm9000_z1` jobs to 0.

### Redeploy CF

In order for the above changes to take effect, you will need to redeploy your cf deployment:
```
pushd $CF_RELEASE_DIR
bosh -d example_manifests/minimal-aws.yml deploy
popd
```

### Fill in the Needed Values in the Diego Manifest

You will need to make the following changes to the example `diego.yml` provided:

- Replace REPLACE_WITH_BOSH_STEMCELL_VERSION with the version of the bosh stemcell uploaded to the director
- Replace REPLACE_WITH_DIRECTOR_ID with the UUID obtained from running `bosh status`
- Replace the `properties.consul` properties that begin with REPLACE_WITH with the values of `properties.consul` from [minimal-aws.yml](https://github.com/cloudfoundry/cf-release/blob/master/example_manifests/minimal-aws.yml).
- Replace the `properties.route_emitter.nats` properties that begin with REPLACE_WITH with the values of `properties.nats` from [minimal-aws.yml](https://github.com/cloudfoundry/cf-release/blob/master/example_manifests/minimal-aws.yml).


### Generate Diego Manifest from Stubs (optional)

You can also use the provided stubs to generate a diego manifest.

You will need to make the following changes to the example stubs provided:

- `stubs/iaas-settings.yml`
  - Replace REPLACE_WITH_DIEGO_SUBNET_AZ with the availability zone for the diego subnet
  - Replace REPLACE_WITH_BOSH_STEMCELL_VERSION with the version of the bosh stemcell uploaded to the director
  - Replace REPLACE_WITH_DIEGO_SUBNET_ID with the subnet-id for the diego subnet
- `stubs/property_overrides.yml`
  - Replace REPLACE_WITH_SSH_HOST_KEY with the contents of `ssh-proxy-host-key.pem`.

After replacing these values you can generate the diego manifest by running:
```
pushd $DIEGO_RELEASE_DIR
  ./scripts/generate-deployment-manifest -c $CF_RELEASE_DIR/example_manifests/minimal-aws.yml \
    -i $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/iaas-settings.yml \
    -p $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/property_overrides.yml \
    -n $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/instance_count_overrides.yml
popd
```

### Upload Diego Bosh Releases

In order to successfully deploy diego, you will need to upload the following bosh releases:

```
bosh upload release https://bosh.io/d/github.com/cloudfoundry/diego-release
bosh upload release https://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-rootfs-release
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-linux-release
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release
```

### Deploy Diego

```
pushd $DIEGO_RELEASE_DIR
bosh -d examples/minimal-aws/diego.yml deploy
popd
```
