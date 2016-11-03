# Optional Configurations for Diego

## Table of Contents

1. [Setup Volume Drivers for Diego](#setup-volume-drivers-for-diego)
1. [Setup Garden Windows for Diego](#setup-garden-windows-for-diego)


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
 bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-windows-bosh-release
```

### Edit the Instance-Count-Overrides Stub

Copy the example stub to `$DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml`:

```bash
cp $DIEGO_RELEASE_DIR/examples/aws/stubs/diego/instance-count-overrides-example.yml $DEPLOYMENT_DIR/stubs/diego-windows/instance-count-overrides.yml
```

Edit that file to change the instance counts of the deployed Diego VMs.

And example instance count overrides stub is below:

```yaml
---
instance_count_overrides:
  cell_windows_z1:
    instances: 5
  cell_windows_z2:
    instances: 0
```

### Edit the Release-Versions Stub

Copy the example release-versions stub to the correct location:

```bash
cp $DIEGO_RELEASE_DIR/examples/aws/stubs/diego/release-versions.yml $DEPLOYMENT_DIR/stubs/diego-windows/release-versions.yml
```

Edit it to change the versions of the Diego and Garden-Windows in
the Diego Windows cell deployment, instead of using the latest versions uploaded to the BOSH
director.

An example release versions stub is below:

```yaml
---
release-versions:
  diego: latest
  garden-windows: latest
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
