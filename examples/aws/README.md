# Deploying CF and Diego to AWS

These instructions allow you to:

* Provision an AWS account with preliminary resources and secrets,
* Deploy BOSH to AWS via `bosh-init`, and
* Deploy CF and Diego via the deployed BOSH.

## Table of Contents

1. [Setting Up the Local Environment](#setting-up-the-local-environment)
1. [Creating the AWS Environment](#creating-the-aws-environment)
1. [Deploying Cloud Foundry](#deploying-cloud-foundry)
1. [Deploying Diego](#deploying-diego)

## Setting Up the Local Environment

### Setting Up Local Dependencies

As part of the deployment process, you must install the following dependencies:

* [Go 1.6](https://golang.org/doc/install)
* [godep](https://github.com/tools/godep)
* [boosh](https://github.com/vito/boosh)
* [spiff](https://github.com/cloudfoundry-incubator/spiff)
* [AWS CLI](https://aws.amazon.com/cli/)
* [jq](https://stedolan.github.io/jq/)
* [ruby](https://www.ruby-lang.org/en/documentation/installation/)
* [BOSH CLI](http://bosh.io/docs/bosh-cli.html)
* [bosh-init](https://bosh.io/docs/install-bosh-init.html)

You must also clone the following git repositories from GitHub:

* [cf-release](https://github.com/cloudfoundry/cf-release)
* [diego-release](https://github.com/cloudfoundry-incubator/diego-release)

### Deployment Directory

The deployment process requires that you create a directory for each deployment
which will hold the necessary configuration to deploy bosh, cf-release, and
diego-release.

### Exporting Directory Locations as Environment Variables

Export the locations of the deployment directory, CF release directory, and Diego release directory
as the following environment variables, replacing the `REPLACE_WITH` placeholders below to match your local paths:

```bash
export DEPLOYMENT_DIR=REPLACE_WITH_PATH_TO_DEPLOYMENT_DIR
export CF_RELEASE_DIR=REPLACE_WITH_PATH_TO_CF_RELEASE_DIR
export DIEGO_RELEASE_DIR=REPLACE_WITH_PATH_TO_DIEGO_RELEASE_DIR
```

These instructions use these environment variables as `$DEPLOYMENT_DIR`, `$CF_RELEASE_DIR`, and `$DIEGO_RELEASE_DIR`.

### AWS Requirements

Before deploying the BOSH director, you must create the following resources in
your AWS account through the AWS console:

#### IAM User Policy

1. From the AWS console homepage, click on `Identity & Access Management`.
1. Click on the `Policies` link.
1. Click on the `Create Policy` button.
1. Select `Create Your Own Policy`.
1. Enter `bosh-aws-policy` as the `Policy Name`.
1. Enter the following as the `Policy Document` and click on the `Create Policy` button:
```json
{
  "Version": "2012-10-17",
    "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "iam:DeleteServerCertificate",
        "iam:UploadServerCertificate",
        "iam:ListServerCertificates",
        "iam:GetServerCertificate",
        "cloudformation:*",
        "ec2:*",
        "s3:*",
        "vpc:*",
        "elasticloadbalancing:*",
        "route53:*"
      ],
      "Resource": "*"
    }
  ]
}
```

#### IAM User

1. From the AWS console homepage, click on `Identity & Access Management`.
1. Click on `Users` link.
1. Click on the `Create New Users` button.
1. Fill in only one user name.
1. Make sure that the `Generate an access key for each user` checkbox is checked and click `Create`.
1. Click `Download Credentials` at the bottom of the screen.
1. Copy the downloaded `credentials.csv` file to `$DEPLOYMENT_DIR`.
1. Click on the `Close` link to return to the IAM Users page.
1. Click on the user that you created.
1. Click on the `Permissions` tab.
1. Click on the `Attach Policy` button.
1. Filter for `bosh-aws-policy` in the filter box
1. Select `bosh-aws-policy` and click on the `Attach Policy` button


#### AWS keypair for the BOSH director

1. From the AWS console homepage, click on `EC2`.
1. Click on the `Key Pairs` link in the sidebar, in the `Network & Security` group.
1. Click the `Create Key Pair` button at the top of the page.
1. When prompted for the key name, enter a name that can be easily referred to later, for example: `bosh_keypair`.
1. Make the directory `$DEPLOYMENT_DIR/keypair` and move the downloaded `bosh_keypair.pem` key to `$DEPLOYMENT_DIR/keypair/id_rsa_bosh`.
1. Change the permissions on the new key file to `600` (`rw-------`): `chmod 600 $DEPLOYMENT_DIR/keypair/id_rsa_bosh`.

#### Route 53 Hosted Zone

1. From the AWS console homepage, click on `Route 53`.
1. Select `Hosted zones` from the left sidebar.
1. Click the `Create Hosted Zone` button.
1. Fill in the domain name you intend to use for your Cloud Foundry deployment. The domain name for your hosted zone will be the base domain for all apps deployed to your Cloud Foundry instance, as well as the base domain for the Cloud Foundry system components. This domain name will be referred to as `$CF_DOMAIN` below.

If you host this domain at another domain registrar, set the nameservers at that registrar to the DNS servers listed in the NS record in the AWS Hosted Zone.

### Deployment Directory Setup

After creating the necessary resources in AWS, you must populate
`$DEPLOYMENT_DIR` in the following format. Each of the files is explained further
below.

```
DEPLOYMENT_DIR
|-(bootstrap_environment)
|-keypair
| |-(id_rsa_bosh)
|-certs
| |-(elb-cfrouter.key)
| |-(elb-cfrouter.pem)
|-stubs
| |-(domain.yml)
| |-infrastructure
| | |-(availablity_zones.yml)
| |-bosh-init
|   |-(keypair.yml)
|   |-(releases.yml)
|   |-(users.yml)
|   |-(stemcell.yml)
```

To create the directories, run the following commands:

```bash
cd $DEPLOYMENT_DIR
mkdir -p keypair
mkdir -p certs
mkdir -p stubs/infrastructure
mkdir -p stubs/bosh-init
```

#### `bootstrap_environment`

This script exports your AWS default region and the access and secret keys of your IAM user as environment variables.
Copy the template below into a new `bootstrap_environment` file in `$DEPLOYMENT_DIR`, then replace the `PLACEHOLDER` values as follows from the values in the `credentials.csv` file downloaded during [creation of the IAM user](#iam-user):

- For the `AWS_ACCESS_KEY_ID` variable, replace `REPLACE_WITH_AKI` with the access key id.
- For the `AWS_SECRET_ACCESS_KEY` variable, replace `REPLACE_WITH_SECRET_ACCESS_KEY` with the secret access key.

```bash
export AWS_DEFAULT_REGION=us-east-1
export AWS_ACCESS_KEY_ID=REPLACE_WITH_AKI
export AWS_SECRET_ACCESS_KEY='REPLACE_WITH_SECRET_ACCESS_KEY'
```

#### `keypair/id_rsa_bosh`

This file is the private key pair generated as the [AWS keypair for the BOSH director](#aws-keypair-for-the-bosh-director).

####<a name="elb-cfrouter"></a> `certs/elb-cfrouter.key` and `certs/elb-cfrouter.pem`

An SSL certificate for the domain where Cloud Foundry will be accessible is required.
If you do not already provide a certificate, you can generate a self-signed certificate following the commands below. 

```
openssl genrsa -out elb-cfrouter.key 2048
```

When prompted for the 'Common Name' in the next command, enter `*.$CF_DOMAIN`, where `$CF_DOMAIN` is the value you entered in the [hosted zone setup](#route-53-hosted-zone). The other fields can be left blank.

```
openssl req -new -key elb-cfrouter.key -out elb-cfrouter.csr
openssl x509 -req -in elb-cfrouter.csr -signkey elb-cfrouter.key -out elb-cfrouter.pem
```

#### `stubs/domain.yml`

Enter the domain for the [Route 53 Hosted Zone](#route-53-hosted-zone) in the `domain.yml` stub:

```yaml
---
domain: $CF_DOMAIN
```

#### `stubs/infrastructure/availability_zones.yml`

This YAML file defines the three availability zones to host your Cloud Foundry deployment.
They must be located in the region specified in the `bootstrap_environment` file. For example:

```yaml
---
meta:
  availability_zones:
  - us-east-1a
  - us-east-1c
  - us-east-1d
```

Note: These zones could become restricted by AWS. If at some point during the `deploy_aws_cli` script and you see an error
similar to the following message:

```
Value (us-east-1b) for parameter availabilityZone is invalid Subnets can currently only be created in the following availability zones: us-east-1d, us-east-1c, us-east-1a, us-east-1e
```

then update this file with acceptable availability zone values.

#### `stubs/bosh-init/keypair.yml`

This YAML file contains the name of the keypair created on [AWS keypair for the
BOSH director](#aws-keypair-for-the-bosh-director). Use the same name that was
used on that step.

```yaml
---
BoshKeypairName: REPLACE_WITH_BOSH_KEYPAIR_NAME
```

For example:

```yaml
---
BoshKeypairName: bosh_keypair
```

#### `stubs/bosh-init/releases.yml`

To deploy the BOSH director, the `releases.yml` for `bosh-init` must specify `bosh` and `bosh-aws-cpi` releases by `url` and `sha1`.
Releases for `bosh` can be found [here](https://bosh.io/releases/github.com/cloudfoundry/bosh?all=1), and 
releases for `bosh-aws-cpi` can be found [here](https://bosh.io/releases/github.com/cloudfoundry-incubator/bosh-aws-cpi-release?all=1).

Fill out the following template with the desired values:

```yaml
---
releases:
- name: bosh
  url: REPLACE_WITH_URL_TO_LATEST_BOSH_BOSH_RELEASE
  sha1: REPLACE_WITH_SHA1_OF_LATEST_BOSH_BOSH_RELEASE
- name: bosh-aws-cpi
  url: REPLACE_WITH_URL_TO_LATEST_BOSH_AWS_CPI_BOSH_RELEASE
  sha1: REPLACE_WITH_SHA1_OF_LATEST_BOSH_AWS_CPI_BOSH_RELEASE
```

For example:

```yaml
releases:
- name: bosh
  url: https://bosh.io/d/github.com/cloudfoundry/bosh?v=255
  sha1: 923dfb8c26fab7041c0a3e591f0e92f3c4bca29e
- name: bosh-aws-cpi
  url: https://bosh.io/d/github.com/cloudfoundry-incubator/bosh-aws-cpi-release?v=44
  sha1: a1fe03071e8b9bf1fa97a4022151081bf144c8bc
```

#### `stubs/bosh-init/users.yml`

This file defines the admin users for the BOSH director. Replace the 'password' field below with the password you intend to use for the 'admin' user.

```yaml
---
BoshInitUsers:
- name: admin
  password: REPLACE_WITH_YOUR_PASSWORD
```

#### `stubs/bosh-init/stemcell.yml`

This file defines which stemcell to use on the BOSH director. Stemcells can be found
[here](https://bosh.io/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent), and must be specified by their `url` and `sha1`.

```yaml
---
BoshInitStemcell:
  url: REPLACE_WITH_URL_TO_LATEST_BOSH_AWS_HVM_STEMCELL
  sha1: REPLACE_WITH_URL_TO_LATEST_BOSH_AWS_HVM_STEMCELL
```

For example:

```yaml
---
BoshInitStemcell:
  url: https://bosh.io/d/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent?v=3197
  sha1: 89a2210b8caf3884855d7db6d48b8863202c7783
```

The [bosh.io](https://bosh.io) site does not currently provide the SHA1 hash of stemcells. You must download the
stemcell locally and calcuate the SHA1 hash manually. On Mac OS X, this can be done on OSX by running:

```
shasum /path/to/downloaded/stemcell
```

### Configuring Security

In order to secure your Cloud Foundry deployment properly, you must generate SSL certificates and keys to secure traffic between components.

The CF and Diego release repositories provide scripts to generate the necessary SSL certificates.

1. To generate certificates for consul, run:
```bash
cd $DEPLOYMENT_DIR/certs
$CF_RELEASE_DIR/scripts/generate-consul-certs
```
1. To generate certificates for the etcd and BBS servers in the Diego deployment, run:
```bash
$DIEGO_RELEASE_DIR/scripts/generate-diego-certs
mv $DIEGO_RELEASE_DIR/diego-certs/* $DEPLOYMENT_DIR/certs
```

After running these scripts, you should see the following files in `$DEPLOYMENT_DIR/certs`:
```
DEPLOYMENT_DIR/certs
|- diego-ca.crt
|- diego-ca.key
|- etcd-peer-ca.crt
|- etcd-peer-ca.key
|-bbs-certs         # generated via diego-release/scripts/generate-diego-certs
|  |- client.crt
|  |- client.key
|  |- server.crt
|  |- server.key
|-consul-certs      # generated via cf-release/scripts/generate-consul-certs
|  |- agent.crt
|  |- agent.key
|  |- server-ca.crt
|  |- server-ca.key
|  |- server.crt
|  |- server.key
|-etcd-certs        # generated via diego-release/scripts/generate-diego-certs
|  |- client.crt
|  |- client.key
|  |- server.crt
|  |- server.key
|  |- peer.crt
|  |- peer.key
```

You can ignore any files with a `crl` or `csr`.

The certificates in `consul-certs` are used to set SSL properties for the consul VMs, and the certificates in `bbs-certs` and `etcd-certs` are used to set SSL properties on the Diego etcd cluster and BBS API servers.

####<a name="generating-ssh-proxy-host-key"></a>Generating SSH Proxy Host Key and Fingerprint

To enable SSH access to CF instances running on Diego, generate a host key and fingerprint for the SSH proxy as follows, entering an empty string for the passphrase when prompted:

```bash
ssh-keygen -f $DEPLOYMENT_DIR/keypair/ssh-proxy-host-key.pem
```

If the local `ssh-keygen` supports the `-E` flag, as it does on OS X 10.11 El Capitan or Ubuntu 16.04 Xenial Xerus, generate the MD5 fingerprint of the public host key as follows:

```bash
ssh-keygen -lf $DEPLOYMENT_DIR/keypair/ssh-proxy-host-key.pem.pub -E md5 | cut -d ' ' -f2 | sed 's/MD5://g' > $DEPLOYMENT_DIR/keypair/ssh-proxy-host-key-fingerprint
```

Otherwise, generate the MD5 fingerprint as follows:

```bash
ssh-keygen -lf $DEPLOYMENT_DIR/keypair/ssh-proxy-host-key.pem.pub | cut -d ' ' -f2 > $DEPLOYMENT_DIR/keypair/ssh-proxy-host-key-fingerprint
```

The `ssh-proxy-host-key.pem` file contains the PEM-encoded private host key for the Diego manifest, and the `ssh-proxy-host-key-fingerprint` file contains the MD5 fingerprint of the public host key. You will later copy these values into stubs for generating the CF and Diego manifests.

#### Generating UAA Private/Public Keys

UAA requires an RSA keypair for its configuration. Generate one as follows, entering an empty string for the passphrase when prompted:

```bash
ssh-keygen -t rsa -b 4096 -f $DEPLOYMENT_DIR/keypair/uaa
openssl rsa -in $DEPLOYMENT_DIR/keypair/uaa -pubout > $DEPLOYMENT_DIR/keypair/uaa.pub
```

## Creating the AWS environment

To create the AWS environment and two VMs essential to the Cloud Foundry infrastructure,
run `./deploy_aws_environment create "$CF_RELEASE_DIR" "$DEPLOYMENT_DIR"`
**from the directory containing these instructions** (`$DIEGO_RELEASE_DIR/examples/aws`).
This process may take up to 30 minutes.

```bash
cd "$DIEGO_RELEASE_DIR/examples/aws"
./deploy_aws_environment create "$CF_RELEASE_DIR" "$DEPLOYMENT_DIR"
```

The `./deploy_aws_environment` script takes three arguments:

- The first argument is one of three directives, which you'll need if our script doesn't succeed the first time:
  - `create` creates an AWS CloudFormation stack based off of the stubs filled out above.
  - `update` updates the CloudFormation stack. Run the script with this command after changing the stubs in `$DEPLOYMENT_DIR/stubs/infrastructure`, or after an update to this example directory. If there are **no** changes to the stack, instead run the `skip` command below, as otherwise the script will fail.
  - `skip` upgrades the BOSH director without affecting the CloudFormation stack.

- The second argument is the **absolute path** to `$CF_RELEASE_DIR`.
- The third argument is the **absolute path** to `$DEPLOYMENT_DIR`, which must be structured as defined above.

The deployment process generates a collection of stubs, in the following directory structure. Some of the stubs start with the line `GENERATED: NO TOUCHING`, and are not intended for hand-editing.

```
DEPLOYMENT_DIR
|-stubs
| |- director-uuid.yml # the unique id of the BOSH directory
| |- aws-resources.yml  # general metadata about the CloudFormation stack
| |-cf
| | |- stub.yml # networks, zones, s3 buckets for the Cloud Foundry deployment
| | |- properties.yml # consul configuration and shared secrets
| | |- domain.yml # domain
| |-diego
| | |- property-overrides.yml # stub to parametrize with Diego manifest property overrides
| | |- iaas-settings.yml # networks, zones for the Diego deployment
| |-infrastructure
|   |- certificates.yml # certificates for the cfrouter ELB
|   |- cloudformation.json # CloudFormation JSON deployed to AWS
|-deployments
| |-bosh-init
|   |- bosh-init.yml # bosh director deployment
```

### `stubs/cf/stub.yml`

The `./deploy_aws_environment` script generates a partial stub for your
Cloud Foundry deployment. It is a generated stub that contains information specific to the AWS CloudFormation stack and should not be edited manually.

### `stubs/cf/properties.yml`

The `./deploy_aws_environment` script copies another partial stub for your
Cloud Foundry deployment. This stub is intended to be editied, as describes in more detail in the
[Manifest Generation](#manifest-generation) section.

### `stubs/diego/property-overrides.yml`

This stub will be used as part of Diego manifest generation and was constructed from
your deployed AWS infrastructure, as well as our default template. This stub provides
the skeleton for the certificates generated in the
[Configuring Security](#configuring-security) section,
as well as for setting the log levels of components.

### `stubs/diego/iaas-settings.yml`

This stub is during Diego manifest generation.
It contains settings specific to your AWS environment.

## Set up Public DNS for BOSH Director (optional)

For your BOSH director to be accessible on the Internet via DNS using the
[Route 53 hosted zone](#route-53-hosted-zone) created above,
perform the following steps:

1. From the EC2 dashboard, obtain the public IP address of the `bosh/0` BOSH director instance.
1. Click on the `Route53` link on the AWS console.
1. Click the `Hosted Zones` link.
1. Click on the hosted zone created earlier.
1. Click the `Create Record Set` button.
1. Enter `bosh` for the `Name`.
1. Change the `Type` to `A - IPv4 address` if it is not already set to that type.
1. Enter the public IP address of the BOSH director for the value.
1. Click the `Create` button.

## [Using RDS MySQL instead of etcd. (OPTIONAL.md#setup-aws-rds-mysql) (optional)

## Deploying Cloud Foundry

### Manifest Generation

To deploy Cloud Foundry, you need a stub similar to the one from the [Cloud Foundry Documentation](http://docs.cloudfoundry.org/deploying/aws/cf-stub.html).
The generated stub `$DEPLOYMENT_DIR/stubs/cf/stub.yml` already has some of these properties filled out for you.
The stub `$DEPLOYMENT_DIR/stubs/cf/properties.yml` contains some additional placeholder properties that you must specify.
For more information on stubs for Cloud Foundry manifest generation, please refer to the documentation [here](http://docs.cloudfoundry.org/deploying/aws/cf-stub.html#editing).

#### Diego Stub for CF

The default deployment configuration from the manifest-generation scripts in cf-release omits some instances and properties that Diego depends on.
It also includes some instances and properties that are unnecessary for a deployment with Diego as the only container runtime. Including `$DIEGO_RELEASE_DIR/examples/aws/stubs/cf/diego.yml` in the list of stubs when generating the Cloud Foundry manifest will configure these instance counts and properties correctly.

#### Fill in Properties Stub

In order to correctly generate a manifest for the Cloud Foundry deployment, you must
replace certain values in the provided `$DEPLOYMENT_DIR/stubs/cf/properties.yml`.
Replace all the values that are prefixed with `REPLACE_WITH_`.

**Note:** If you did not generate a self-signed certificate for the
[CF Router ELB](#elb-cfrouter) and are instead using a certificate signed by a
trusted certificate authority, change the value of `properties.ssl.skip_cert_verify`
from `true` to `false`.

If you also wish to change the instance counts for the jobs in the CF deployment, add those different counts to this stub. These counts will override the counts set in the `$DIEGO_RELEASE_DIR/examples/aws/stubs/cf/diego.yml` if using the command below to generate the manifest.

#### Generate the CF deployment manifest

After following the instructions to fill out the placeholder values
in the `DEPLOYMENT_DIR/stubs/cf/stub.yml` stub, run the following to generate the Cloud Foundry manifest:

```bash
cd $CF_RELEASE_DIR
./scripts/generate_deployment_manifest aws \
  $DEPLOYMENT_DIR/stubs/director-uuid.yml \
  $DIEGO_RELEASE_DIR/examples/aws/stubs/cf/diego.yml \
  $DEPLOYMENT_DIR/stubs/cf/properties.yml \
  $DEPLOYMENT_DIR/stubs/cf/stub.yml \
  > $DEPLOYMENT_DIR/deployments/cf.yml
```

### Target the BOSH Director

Target the BOSH director using either the public IP address or the Route53 record created earlier.
The public IP address can be obtained from either the `$DEPLOYMENT_DIR/stubs/aws-resources.yml`
under `Resources.EIP.BoshInit` or from the EC2 dashboard in the AWS console.

```bash
bosh target bosh.$CF_DOMAIN
```

When prompted for the username and password, provide the credentials set in the `$DEPLOYMENT_DIR/stubs/bosh-init/users.yml` stub.

### Upload the BOSH Stemcell

Upload the lastest BOSH stemcell for AWS to the bosh director.
You can find the latest stemcell [here](http://bosh.io/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent).

```bash
bosh upload stemcell /path/to/stemcell
```

### Create and Upload the CF Release

In order to deploy CF, create and upload the release to the director using the following commands:

```bash
cd $CF_RELEASE_DIR
bosh --parallel 10 create release
bosh upload release
```

### Deploy

Set the CF deployment manifest and deploy with the following commands:

```bash
bosh deployment $DEPLOYMENT_DIR/deployments/cf.yml
bosh deploy
```

From here, follow the documentation on [deploying a Cloud Foundry with BOSH](http://docs.cloudfoundry.org/deploying/common/deploy.html). Depending on the size of the deployment and the time required for package compilation, the initial deploy can take many minutes or hours.

## Deploying Diego

After deploying Cloud Foundry, you can now deploy Diego.

### Fill in the Property-Overrides Stub

To generate a manifest for the Diego deployment, replace the properties in
`$DEPLOYMENT_DIR/stubs/diego/property-overrides.yml` file that are prefixed with `REPLACE_WITH_`.

Here is a summary of the properties that need to be changed:

- Replace REPLACE_WITH_ACTIVE_KEY_LABEL with any desired key name (such as `key-a`).
- Replace REPLACE_WITH_A_SECURE_PASSPHRASE with a unique passphrase associated with the active key label.
- Replace the BBS and etcd certificate placeholders with the contents of the files generated in [Configuring Security](#configuring-security).
- Replace the SSH-Proxy host key with the [host key generated](#generating-ssh-proxy-host-key) above.

### Edit the Instance-Count-Overrides Stub

Copy the example stub to `$DEPLOYMENT_DIR`:

```bash
cp $DIEGO_RELEASE_DIR/examples/aws/stubs/diego/instance-count-overrides-example.yml $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml
```

That stub can be edited if there's need to change the instance counts for any of the jobs deployed.

### Edit the Release-Versions Stub

Copy the example release-versions stub to the correct location:

```bash
cp $DIEGO_RELEASE_DIR/examples/aws/stubs/diego/release-versions.yml $DEPLOYMENT_DIR/stubs/diego/release-versions.yml
```

Edit it to fix the versions of the Diego, Garden-Linux, and etcd releases in
the Diego deployment, instead of using the latest versions uploaded to the BOSH
director.

For example, to use version 22 of etcd-release and version 0.331.0 of garden-linux-release, edit the stub to read:

```yaml
release-versions:
  diego: latest
  etcd: 22
  garden_linux: 0.331.0
  cflinuxfs2_rootfs: 0.2.0
```

### [Fill in diego-sql Stub](OPTIONAL.md#fill-in-diego-sql-stub) (optional)

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
  > $DEPLOYMENT_DIR/deployments/diego.yml
```

Optionally use instructions for [generating With SQL Backend](OPTIONAL.md#generate-the-diego-manifest)

### Upload Garden-Linux, etcd, and cflinuxfs2 releases

1. Upload the latest garden-linux-release:
    ```bash
    bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-linux-release
    ```

    To upload a specific version of garden-linux-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry-incubator/garden-linux-release).

1. Upload the latest etcd-release:
    ```bash
    bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release
    ```

    To upload a specific version of etcd-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry-incubator/etcd-release).

1. Upload the latest cflinuxfs2-rootfs-release:
    ```bash
    bosh upload release https://bosh.io/d/github.com/cloudfoundry/cflinuxfs2-rootfs-release
    ```

    To upload a specific version of cflinuxfs2-rootfs-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry/cflinuxfs2-rootfs-release).

### Deploy Diego

As with the Cloud Foundry deployment, once the Diego manifest is generated, you need to create the Diego release, upload it to the BOSH director, and deploy Diego:

```bash
bosh deployment $DEPLOYMENT_DIR/deployments/diego.yml
cd $DIEGO_RELEASE_DIR
bosh --parallel 10 create release --force
bosh upload release
bosh deploy
```
