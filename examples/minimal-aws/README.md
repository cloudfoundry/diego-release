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
    - Update the security group for this NAT instance to add the following rule:
      - Type: All traffic
      - Protocol: All
      - Port Range: 0 - 65535
      - Source: Custom IP / 10.0.18.0/24
- Click "Save"

### Generate SSH Proxy Host Key and Fingerprint

Run the following to generate a host key for the SSH proxy. When prompted for the key passphrase, hit 'enter' to set no passphrase.

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

Replace REPLACE_WITH_SSH_HOST_KEY in the example diego manifest with the contents of `ssh-proxy-host-key.pem`, making sure to indent the contents correctly for the YAML literal block.

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
      ssh-proxy:
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

You will need to make the following changes to the example `diego.yml` file provided:

- Replace REPLACE_WITH_BOSH_STEMCELL_VERSION with the version of the BOSH stemcell uploaded to the director.
- Replace REPLACE_WITH_DIRECTOR_ID with the UUID obtained from running `bosh status`.

Some of the values come from the CF manifest constructed by modifying [minimal-aws.yml](https://github.com/cloudfoundry/cf-release/blob/master/example_manifests/minimal-aws.yml):

- Replace the `properties.consul` properties that begin with REPLACE_WITH with the values of `properties.consul` from your CF manifest.
- Replace the `properties.route_emitter.nats` properties that begin with REPLACE_WITH with the values of `properties.nats` from your CF manifest.
- Replace the REPLACE_WITH_ETCD_MACHINES_FROM_CF value in the `properties.loggregator.etcd.machines` property with the value of `properties.loggregator.etcd.machines` from your CF manifest.


### Generate Diego Manifest from Stubs (optional)

You can also use the provided stubs to generate a diego manifest.

You will need to make the following changes to the example stubs provided:

- `stubs/diego/iaas-settings.yml`
  - Replace REPLACE_WITH_DIEGO_SUBNET_AZ with the availability zone for the diego subnet
  - Replace REPLACE_WITH_BOSH_STEMCELL_VERSION with the version of the bosh stemcell uploaded to the director
  - Replace REPLACE_WITH_DIEGO_SUBNET_ID with the subnet-id for the diego subnet
- `stubs/diego/property_overrides.yml`
  - Replace REPLACE_WITH_SSH_HOST_KEY with the contents of `ssh-proxy-host-key.pem`.

After replacing these values you can generate the diego manifest by running:
```
pushd $DIEGO_RELEASE_DIR
  ./scripts/generate-deployment-manifest -c $CF_RELEASE_DIR/example_manifests/minimal-aws.yml \
    -i $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/diego/iaas-settings.yml \
    -p $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/diego/property_overrides.yml \
    -n $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/diego/instance_count_overrides.yml
popd
```

### Upload Diego Bosh Releases

In order to successfully deploy diego, you will need to upload the following bosh releases:

```
bosh upload release https://bosh.io/d/github.com/cloudfoundry/diego-release
bosh upload release https://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-rootfs-release
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-runc-release
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release
```

### Deploy Diego

```
pushd $DIEGO_RELEASE_DIR
bosh -d examples/minimal-aws/diego.yml deploy
popd
```

### Deploy Diego Windows Cells (optional)

### Create a Diego Windows Subnet

- Click on "Subnets" from the VPC Dashboard
- Click "Create Subnet"
- Fill in
  - Name tag: diego_windows
  - VPC: bosh
  - Availability Zone: Pick the same Availability Zone as the bosh Subnet
    - Replace REPLACE_WITH_DIEGO_WINDOWS_SUBNET_AZ in the example manifest with the Availability Zone you chose
  - CIDR block: 10.0.20.0/24
  - Click "Yes, Create"
- Replace REPLACE_WITH_DIEGO_WINDOWS_SUBNET_ID in the example manifest with the Subnet ID for the diego Subnet
- Select the `diego_windows` Subnet from the Subnet list
- Click the name of the "Route table:" to view the route tables
- Select the route table from the list
- Click "Routes" in the bottom window
- Click "Edit"
- Fill in a new route
  - Destination: 0.0.0.0/0
  - Target: Select the NAT instance from the list
    - Update the security group for this NAT instance to add the following rule:
      - Type: All traffic
      - Protocol: All
      - Port Range: 0 - 65535
      - Source: Custom IP / 10.0.20.0/24
- Click "Save"

### Upload Windows Stemcell

You will need to download the windows bosh stemcell and upload it to the bosh director with the following:

```
wget https://s3.amazonaws.com/bosh-windows-stemcells/light-bosh-stemcell-0.0.50-aws-xen-hvm-windows2012R2-go_agent.tgz
bosh upload stemcell light-bosh-stemcell-0.0.50-aws-xen-hvm-windows2012R2-go_agent.tgz
```


### Fill in the Needed Values in the Diego Windows Cells Manifest

You will need to make the following changes to the example `diego_windows_cells.yml` file provided:

- Replace REPLACE_WITH_BOSH_WINDOWS_STEMCELL_VERSION with the version of the BOSH stemcell uploaded to the director.
- Replace REPLACE_WITH_DIRECTOR_ID with the UUID obtained from running `bosh status`.

Some of the values come from the CF manifest constructed by modifying [minimal-aws.yml](https://github.com/cloudfoundry/cf-release/blob/master/example_manifests/minimal-aws.yml):

- Replace the `properties.consul` properties that begin with REPLACE_WITH with the values of `properties.consul` from your CF manifest.
- Replace the REPLACE_WITH_ETCD_MACHINES_FROM_CF value in the `properties.loggregator.etcd.machines` property with the value of `properties.loggregator.etcd.machines` from your CF manifest.


### Generate Diego Windows Cells Manifest from Stubs (optional)

You can also use the provided stubs to generate a diego windows cells deployment manifest.

You will need to make the following changes to the example stubs provided:

- `stubs/diego-windows/iaas-settings.yml`
  - Replace REPLACE_WITH_DIEGO_WINDOWS_SUBNET_AZ with the availability zone for the diego_windows subnet
  - Replace REPLACE_WITH_BOSH_WINDOWS_STEMCELL_VERSION with the version of the bosh stemcell uploaded to the director
  - Replace REPLACE_WITH_DIEGO_WINDOWS_SUBNET_ID with the subnet-id for the diego_windows subnet

After replacing these values you can generate the diego windows cell manifest by running:
```
pushd $DIEGO_RELEASE_DIR
  ./scripts/generate-windows-cell-deployment-manifest -c $CF_RELEASE_DIR/example_manifests/minimal-aws.yml \
    -i $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/diego-windows/iaas-settings.yml \
    -p $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/diego/property-overrides.yml \
    -n $DIEGO_RELEASE_DIR/examples/minimal-aws/stubs/diego-windows/instance-count-overrides.yml
popd
```

### Upload Garden Windows Bosh Release

In order to successfully deploy Diego Windows cells, you will need to upload the following bosh release:

```
bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-windows-bosh-release
```

### Deploy Diego Windows Cells

```
pushd $DIEGO_RELEASE_DIR
bosh -d examples/minimal-aws/diego_windows_cells.yml deploy
popd
```
