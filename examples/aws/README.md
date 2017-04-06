# Deploying CF and Diego to AWS

These instructions allow you to:

* Provision an AWS account with preliminary resources and secrets,
* Deploy BOSH to AWS via the v2 BOSH CLI, and
* Deploy CF and Diego via the deployed BOSH.

## Table of Contents

1. [Setting Up the Local Environment](#setting-up-the-local-environment)
1. [Creating the AWS Environment](#creating-the-aws-environment)
1. [Deploying Cloud Foundry](#deploying-cloud-foundry)
1. [Setup a SQL database for Diego](#setup-a-sql-database-for-diego)
1. [Deploying Diego](#deploying-diego)

## Setting Up the Local Environment

### Setting Up Local Dependencies

As part of the deployment process, you must install the following dependencies:

* [Go 1.7.1](https://golang.org/doc/install)
* [godep](https://github.com/tools/godep)
* [boosh](https://github.com/vito/boosh)
* [spiff](https://github.com/cloudfoundry-incubator/spiff)
* [AWS CLI](https://aws.amazon.com/cli/)
* [jq](https://stedolan.github.io/jq/)
* [ruby](https://www.ruby-lang.org/en/documentation/installation/)
* [BOSH CLI v2](http://bosh.io/docs/cli-v2.html)

You must also clone the following git repositories from GitHub:

* [bosh-deployment](https://github.com/cloudfoundry/bosh-deployment)
* [cf-release](https://github.com/cloudfoundry/cf-release)
* [diego-release](https://github.com/cloudfoundry/diego-release)

### Deployment Directory

The deployment process requires that you create a directory for each deployment
which will hold the necessary configuration to deploy BOSH, cf-release, and
diego-release.

### Base Domain for Deployment

Before proceeding with setup, select a domain name you intend to use for your CF deployment.
This domain name will be the base domain for all apps deployed to your Cloud Foundry instance, as well as the base domain for the Cloud Foundry system components.
You will later create a Route 53 Hosted Zone for this domain to set up DNS entries for the deployment, so you should make sure you have access at your domain registrar to integrate these DNS settings into your domain.

### Exporting Directory Locations and Configuration as Environment Variables

Change into the directory you just created for the deployment and run the following to produce the `deployment-env` file:

```bash
cat <<"EOF" > deployment-env
export DEPLOYMENT_DIR="$(cd $(dirname "$BASH_SOURCE[0]") && pwd)"

export BOSH_DEPLOYMENT_DIR="$HOME/workspace/bosh-deployment"
export CF_RELEASE_DIR="$HOME/workspace/cf-release"
export DIEGO_RELEASE_DIR="$HOME/workspace/diego-release"

export CF_DOMAIN=REPLACE_WITH_DEPLOYMENT_DOMAIN
export STACK_NAME=REPLACE_WITH_STACK_NAME
export gobosh=bosh

echo "DEPLOYMENT_DIR set to '$DEPLOYMENT_DIR'"
echo "BOSH_DEPLOYMENT_DIR set to '$BOSH_DEPLOYMENT_DIR'"
echo "CF_RELEASE_DIR set to '$CF_RELEASE_DIR'"
echo "DIEGO_RELEASE_DIR set to '$DIEGO_RELEASE_DIR'"
echo "CF_DOMAIN set to '$CF_DOMAIN'"
echo "v2 BOSH CLI located at '$(which "${gobosh}")'"
EOF
```

Edit the `deployment-env` file to replace `REPLACE_WITH_DEPLOYMENT_DOMAIN` with the domain selected above and to replace `REPLACE_WITH_STACK_NAME` with an identifying name to give to the CloudFormation stack. If you have not checked out cf-release and diego-release as subdirectories of `~/workspace`, also replace those default locations.

If the v2 BOSH CLI executable is not available on your PATH at `bosh`, set the `gobosh` environment variable above to its name or path.

Run `source deployment-env` to export these variables to your environment. They will be used extensively as `$DEPLOYMENT_DIR`, `$BOSH_DEPLOYMENT_DIR`, `$CF_RELEASE_DIR`, `$DIEGO_RELEASE_DIR`,  `$CF_DOMAIN`, and `$STACK_NAME` in commands and references below. The deploy script will also respect the `gobosh` environment variable to locate the v2 BOSH CLI.

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
1. Fill in the `$CF_DOMAIN` domain name you chose above for your Cloud Foundry deployment.

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
|-ops-files
| |-bosh
|   |-(aws-cpi-version.yml) [OPTIONAL]
|   |-(bosh-version.yml) [OPTIONAL]
|   |-(stemcell.yml) [OPTIONAL]
|-stubs
| |-(domain.yml)
| |-(aws-instance-types.yml) [OPTIONAL]
| |-bosh
| | |-(datadog.yml) [OPTIONAL]
| | |-(domain.yml) [OPTIONAL]
| | |-(vars.yml)
| |-infrastructure
|   |-(availablity_zones.yml)
```

To create the directories, run the following commands:

```bash
cd $DEPLOYMENT_DIR
mkdir -p certs
mkdir -p keypair
mkdir -p ops-files/bosh
mkdir -p stubs/bosh
mkdir -p stubs/infrastructure
```

#### `bootstrap_environment`

This script exports your AWS default region and the access and secret keys of your IAM user as environment variables.
Run the following to create a new `bootstrap_environment` file in `$DEPLOYMENT_DIR`:

```bash
cat <<EOF > $DEPLOYMENT_DIR/bootstrap_environment
export AWS_DEFAULT_REGION=us-east-1
export AWS_ACCESS_KEY_ID=REPLACE_WITH_AKI
export AWS_SECRET_ACCESS_KEY='REPLACE_WITH_SECRET_ACCESS_KEY'
EOF
```

Next, replace the values prefixed with `REPLACE_WITH_` as follows from the values in the `credentials.csv` file downloaded during [creation of the IAM user](#iam-user):

- For the `AWS_ACCESS_KEY_ID` variable, replace `REPLACE_WITH_AKI` with the access key id.
- For the `AWS_SECRET_ACCESS_KEY` variable, replace `REPLACE_WITH_SECRET_ACCESS_KEY` with the secret access key.

Replace the value of `AWS_DEFAULT_REGION` if you are deploying to a different AWS region.

#### `keypair/id_rsa_bosh`

This file is the private key pair generated as the [AWS keypair for the BOSH director](#aws-keypair-for-the-bosh-director).

####<a name="elb-cfrouter"></a> `certs/elb-cfrouter.key` and `certs/elb-cfrouter.pem`

An SSL certificate for the domain where Cloud Foundry will be accessible is required.
If you do not already provide a certificate, you can generate a self-signed certificate following the commands below.

```
cd $DEPLOYMENT_DIR/certs
openssl genrsa -out elb-cfrouter.key 2048
```

When prompted for the 'Common Name' in the next command, enter `*.$CF_DOMAIN`, where `$CF_DOMAIN` is the value you entered in the [hosted zone setup](#route-53-hosted-zone). The other fields can be left blank.

```
openssl req -new -key elb-cfrouter.key -out elb-cfrouter.csr
openssl x509 -req -in elb-cfrouter.csr -signkey elb-cfrouter.key -out elb-cfrouter.pem
```

#### `stubs/domain.yml`

Run the following command to produce the `stubs/domain.yml` stub file with the domain you [selected above](#base-domain-for-deployment):

```yaml
cat <<EOF > $DEPLOYMENT_DIR/stubs/domain.yml
---
domain: $CF_DOMAIN
EOF
```

#### `stubs/aws-instance-types.yml` [OPTIONAL]

To override the existing resource pool instance sizes for the CF and Diego VMS, you may optionally create a stubs/aws-instance-types.yml file. For example, to make all CF and Diego VMs use the t2.micro instance type, run the following command:

```bash
cat <<EOF > $DEPLOYMENT_DIR/stubs/aws-instance-types.yml
instance_types:
  small: t2.micro
  medium: t2.micro
  large: t2.micro
  runner: t2.micro
  router: t2.micro
  small_errand: t2.micro
  xlarge_errand: t2.micro
  access: t2.micro
  brain: t2.micro
  cc_bridge: t2.micro
  cell: t2.micro
  cell_windows: t2.micro
  database: t2.micro
  route_emitter: t2.micro
EOF
```

If this file does not exist, the default sizes will be used. If the file does exist, but not all types are provided, the default sizes will be used for anything not explicitly overridden.
Note: this file needs to be created before `deploy_aws_environment` script described below is run.

#### `stubs/infrastructure/availability_zones.yml`

Run the following to produce the `stubs/infrastructure/availability_zones.yml` file, which defines the three availability zones to host your Cloud Foundry deployment.

```bash
cat <<EOF > $DEPLOYMENT_DIR/stubs/infrastructure/availability_zones.yml
---
meta:
  availability_zones:
  - us-east-1a
  - us-east-1c
  - us-east-1d
EOF
```

If you wish to use different availability zones, or to assign them a different order, edit this file to replace them. Note that these availability zones must be located in the AWS region specified in the `bootstrap_environment` file.

Note: These zones could become restricted by AWS. If at some point during the `deploy_aws_cli` script and you see an error
similar to the following message:

```
Value (us-east-1b) for parameter availabilityZone is invalid Subnets can currently only be created in the following availability zones: us-east-1d, us-east-1c, us-east-1a, us-east-1e
```

then update this file with acceptable availability zone values.


#### `stubs/bosh/vars.yml`

To configure the SSH keypair and director name used for the BOSH deployment, run the following:

```bash
cat <<EOF > $DEPLOYMENT_DIR/stubs/bosh/vars.yml
---
default_key_name: REPLACE_WITH_BOSH_SSH_KEYPAIR_NAME
director_name: REPLACE_WITH_DIRECTOR_NAME
EOF
```

Next, edit this file to replace `REPLACE_WITH_BOSH_SSH_KEYPAIR_NAME` with the
name of the keypair created on
[AWS keypair for the BOSH director](#aws-keypair-for-the-bosh-director) and to
replace `REPLACE_WITH_DIRECTOR_NAME` with the desired name for the director.

#### `stubs/bosh/datadog.yml` [OPTIONAL]

To configure BOSH to report instance and deployment metrics to Datadog, run the following:

```bash
cat <<EOF > $DEPLOYMENT_DIR/stubs/bosh/datadog.yml
---
datadog_api_key: REPLACE_WITH_DATADOG_API_KEY
datadog_application_key: REPLACE_WITH_DATADOG_APPLICATION_KEY
EOF
```

Next, edit the resulting file to replace the `REPLACE_` placeholder values with a Datadog API key and application key.


#### `stubs/bosh/domain.yml` [OPTIONAL]

To use a DNS name to access the BOSH director, run the following:

```bash
cat <<EOF > $DEPLOYMENT_DIR/stubs/bosh/domain.yml
---
director_domain: REPLACE_WITH_DIRECTOR_DOMAIN_NAME
EOF
```

Next, edit the resulting file to replace the `REPLACE_WITH_DIRECTOR_DOMAIN_NAME` value with the desired domain name. This domain name will be used in the director's certificate for TLS communication.


#### `ops-files/bosh/aws-cpi-version.yml` [OPTIONAL]

Run the following to use a different version of the AWS CPI than the one in the [bosh-deployment](https://github.com/cloudfoundry/bosh-deployment) repository. Final releases for the `bosh-aws-cpi` release can be found on [bosh.io](https://bosh.io/releases/github.com/cloudfoundry-incubator/bosh-aws-cpi-release?all=1).

```bash
cat <<EOF > $DEPLOYMENT_DIR/ops-files/bosh/aws-cpi-version.yml
---
- type: replace
  path: /releases/name=bosh-aws-cpi/url
  value: REPLACE_WITH_URL_TO_BOSH_AWS_CPI_BOSH_RELEASE

- type: replace
  path: /releases/name=bosh-aws-cpi/sha1
  value: REPLACE_WITH_SHA1_OF_BOSH_AWS_CPI_BOSH_RELEASE

- type: replace
  path: /releases/name=bosh-aws-cpi/version
  value: REPLACE_WITH_VERSION_OF_BOSH_AWS_CPI_BOSH_RELEASE
EOF
```

Next, edit the resulting file to replace the `REPLACE_` placeholder values. For example:

```yaml
---
- type: replace
  path: /releases/name=bosh-aws-cpi/url
  value: https://bosh.io/d/github.com/cloudfoundry-incubator/bosh-aws-cpi-release?v=62

- type: replace
  path: /releases/name=bosh-aws-cpi/sha1
  value: f36967927ceae09e5663a41fdda199edfe649dc6

- type: replace
  path: /releases/name=bosh-aws-cpi/version
  value: 62
```


#### `ops-files/bosh/bosh-version.yml` [OPTIONAL]

Run the following to use a different version of BOSH than the one in the [bosh-deployment](https://github.com/cloudfoundry/bosh-deployment) repository. Final releases for the `bosh` release can be found [here](https://bosh.io/releases/github.com/cloudfoundry/bosh?all=1).

```bash
cat <<EOF > $DEPLOYMENT_DIR/ops-files/bosh/bosh-version.yml
---
- type: replace
  path: /releases/name=bosh/url
  value: REPLACE_WITH_URL_TO_BOSH_RELEASE

- type: replace
  path: /releases/name=bosh/sha1
  value: REPLACE_WITH_SHA1_OF_BOSH_RELEASE
EOF
```

Next, edit the resulting file to replace the `REPLACE_` placeholder values. For example:

```yaml
---
- type: replace
  path: /releases/name=bosh/url
  value: https://s3.amazonaws.com/bosh-compiled-release-tarballs/release-bosh-260-on-ubuntu-trusty-stemcell-3312.12-20161220201002.tgz

- type: replace
  path: /releases/name=bosh/sha1
  value: bc569944975482889084addda9be36fca8dafad2
```


#### `ops-files/bosh/stemcell.yml` [OPTIONAL]

Run the following to use a different version of the AWS HVM stemcell than the one in the [bosh-deployment](https://github.com/cloudfoundry/bosh-deployment) repository:

```bash
cat <<EOF > $DEPLOYMENT_DIR/ops-files/bosh/stemcell.yml
---
- type: replace
  path: /resource_pools/name=vms/stemcell/url
  value: REPLACE_WITH_URL_TO_BOSH_AWS_HVM_STEMCELL

- type: replace
  path: /resource_pools/name=vms/stemcell/sha1
  value: REPLACE_WITH_SHA1_OF_BOSH_AWS_HVM_STEMCELL
EOF
```

Next, select an [AWS Xen-HVM Light stemcell](https://bosh.io/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent) and replace the `REPLACE_WITH_` values with the URL and SHA1 checksum. For example:

```yaml
---
- type: replace
  path: /resource_pools/name=vms/stemcell/url
  value: https://bosh.io/d/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent?v=3312.12

- type: replace
  path: /resource_pools/name=vms/stemcell/sha1
  value: 336160ec113edf6f019f997ead2ee586ac716ae6
```

#### Additional Ops Files [OPTIONAL]

Any other files in the `$DEPLOYMENT_DIR/ops-files/bosh` directory will be applied to the BOSH deployment manifest as ops-files when it is generated. If those ops files require additional variables, they can be specified as additional entries in the `vars.yml` file above.


### Configuring Security

In order to secure your Cloud Foundry deployment properly, you must generate SSL certificates and keys to secure traffic between components.

The CF and Diego release repositories provide scripts to generate the necessary SSL certificates.

1. To generate certificates for consul and cloud controller run:
```bash
cd $DEPLOYMENT_DIR/certs
$CF_RELEASE_DIR/scripts/generate-cf-diego-certs
$CF_RELEASE_DIR/scripts/generate-consul-certs
```

1. To generate certificates for uaa run:

```bash
pushd $CF_RELEASE_DIR
  ./scripts/generate-uaa-certs
  mv uaa-certs/ $DEPLOYMENT_DIR/certs/
popd

pushd $DIEGO_RELEASE_DIR
  ./scripts/generate-uaa-saml-certs   # output will be in diego-certs/uaa-saml-certs
  mv diego-certs/uaa-saml-certs/saml.* $DEPLOYMENT_DIR/certs/uaa-certs/
popd
```

1. To generate certificates for loggregator run:
```bash
$CF_RELEASE_DIR/scripts/generate-loggregator-certs $DEPLOYMENT_DIR/certs/cf-diego-certs/cf-diego-ca.crt $DEPLOYMENT_DIR/certs/cf-diego-certs/cf-diego-ca.key

pushd $CF_RELEASE_DIR
  $CF_RELEASE_DIR/scripts/generate-statsd-injector-certs $DEPLOYMENT_DIR/certs/loggregator-certs/loggregator-ca.crt $DEPLOYMENT_DIR/certs/loggregator-certs/loggregator-ca.key
  mv $CF_RELEASE_DIR/statsd-injector-certs $DEPLOYMENT_DIR/certs
popd
```

1. To generate certificates for BBS servers and CC bridge jobs in the Diego deployment, run:
```bash
$DIEGO_RELEASE_DIR/scripts/generate-diego-certs $DEPLOYMENT_DIR/certs/cf-diego-certs
mv $DIEGO_RELEASE_DIR/diego-certs/* $DEPLOYMENT_DIR/certs
```

After running these scripts, you should see the following files in `$DEPLOYMENT_DIR/certs`:
```
DEPLOYMENT_DIR/certs
|- auctioneer-certs       # generated via diego-release/scripts/generate-diego-certs
|  |- client.crt
|  |- client.key
|  |- server.crt
|  |- server.key
|- bbs-certs              # generated via diego-release/scripts/generate-diego-certs
|  |- client.crt
|  |- client.key
|  |- server.crt
|  |- server.key
|- cc-uploader-certs      # generated via diego-release/scripts/generate-diego-certs
|  |- cc
|  |  |- client.crt
|  |  |- client.key
|  |- server.crt
|  |- server.key
|- cf-diego-certs         # generated via cf-release/scripts/generate-cf-diego-certs
|  |- cf-diego-ca.crt
|  |- cf-diego-ca.key
|  |- cloud-controller.crt
|  |- cloud-controller.key
|- consul-certs           # generated via cf-release/scripts/generate-consul-certs
|  |- agent.crt
|  |- agent.key
|  |- server-ca.crt
|  |- server-ca.key
|  |- server.crt
|  |- server.key
|- locket-certs           # generated via diego-release/scripts/generate-diego-certs
|  |- server.crt
|  |- server.key
|- loggregator-certs      # generated via cf-release/scripts/generate-loggregator-certs
|  |- doppler.crt
|  |- doppler.key
|  |- loggregator-ca.crt
|  |- metron.crt
|  |- metron.key
|  |- syslogdrainbinder.crt
|  |- syslogdrainbinder.key
|  |- trafficcontroller.crt
|  |- trafficcontroller.key
|- rep-certs              # generated via diego-release/scripts/generate-diego-certs
|  |- client.crt
|  |- client.key
|  |- server.crt
|  |- server.key
|- statsd-injector-certs  # generated via cf-release/scripts/generate-statsd-injector-certs
|  |- statsdinjector.crt
|  |- statsdinjector.key
|- tps-certs              # generated via diego-release/scripts/generate-diego-certs
|  |- client.crt
|  |- client.key
|- uaa-certs
|  |- saml.crt            # generated via diego-release/scripts/generate-uaa-saml-certs
|  |- saml.key
|  |- saml.key.password
|  |- server.crt          # generated via cf-release/scripts/generate-uaa-certs
|  |- server.key
```

You can ignore any files with a `crl` or `csr` extension.

- The certificates in `auctioneer-certs` are used to secure communication between the BBS and the Auctioneer.
- The certificates in `bbs-certs` are used to set TLS properties on the BBS API servers.
- The certificates in `cc-uploader-certs` are used to set TLS properties for the CC-Uploader component on the CC-Bridge.
- The certificates in `cf-diego-certs` are used to set TLS properties for communication between CF and Diego.
- The certificates in `consul-certs` are used to set TLS properties for the Consul servers and agents.
- The certificates in `locket-certs` are used to set TLS properties on the Locket API servers.
- The certificates in `loggregator-certs` and `statsd-injector-certs` are used to set TLS properties for the Loggregator subsystem.
- The certificates in `rep-certs` are used to secure communication between the Auctioneer, the BBS, and the Cell Rep.
- The certificates in `tps-certs` are used to set TLS properties for the TPS-Watcher component on the CC-Bridge.
- The certificates in `uaa-certs` are used to set TLS properties for the UAA subsystem.

#### <a name="generating-ssh-proxy-host-key"></a>Generating SSH Proxy Host Key and Fingerprint

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
run `./deploy_aws_environment create-stack deploy-bosh "$BOSH_DEPLOYMENT_DIR" "$DEPLOYMENT_DIR" "$STACK_NAME"`
**from the directory containing these instructions** (`$DIEGO_RELEASE_DIR/examples/aws`).
This process may take up to 30 minutes.

```bash
cd "$DIEGO_RELEASE_DIR/examples/aws"
./deploy_aws_environment create-stack deploy-bosh "$BOSH_DEPLOYMENT_DIR" "$DEPLOYMENT_DIR" "$STACK_NAME"
```

The `./deploy_aws_environment` script takes five required arguments:

- The first argument is one of three directives, which you'll need if our script doesn't succeed the first time:
  - `create-stack` creates an AWS CloudFormation stack based off of the stubs filled out above.
  - `update-stack` updates the CloudFormation stack. Run the script with this command after changing the stubs in `$DEPLOYMENT_DIR/stubs/infrastructure`, or after an update to this example directory. If there are **no** changes to the stack, instead run the `skip-stack` command below, as otherwise the script will fail.
  - `skip-stack` upgrades the BOSH director without affecting the CloudFormation stack.

- The second argument is the action to take on the BOSH deployment:
  - `deploy-bosh` uses the BOSH CLI to deploy a new or to re-deploy an existing BOSH director.
  - `skip-bosh` leaves the existing BOSH director deployment unchanged.

- The third argument is the **absolute path** to `$BOSH_DEPLOYMENT_DIR`, the local directory containing the bosh-deployment repository.
- The fourth argument is the **absolute path** to `$DEPLOYMENT_DIR`, the directory containing the configuration files discussed above.
- The fifth argument is the name for the CloudFormation stack that the script creates or updates.

The deployment process generates a collection of stubs, in the following directory structure. Some of the stubs start with the line `GENERATED: NO TOUCHING`, and are not intended for hand-editing.

```
DEPLOYMENT_DIR
|-stubs
| |- director-uuid.yml # the unique id of the BOSH director
| |- aws-resources.yml  # general metadata about the CloudFormation stack
| |-bosh
| | |- aws.yml # AWS resource information for the BOSH deployment
| |-cf
| | |- stub.yml # networks, zones, s3 buckets for the Cloud Foundry deployment
| | |- properties.yml # consul configuration and shared secrets
| | |- domain.yml # domain
| |-diego
| | |- property-overrides.yml # stub to parametrize with Diego manifest property overrides
| | |- iaas-settings.yml # networks, zones for the Diego deployment
| |-diego-windows
| | |- iaas-settings.yml #networks, zones for the Diego Windows deployment
| |-infrastructure
|   |- certificates.yml # certificates for the cfrouter ELB
|   |- cloudformation.json # CloudFormation JSON deployed to AWS
|-deployments
| |-bosh
|   |- bosh.yml # BOSH director deployment
|   |- creds.yml # auto-generated credentials and other variables
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

This stub is used during Diego manifest generation.
It contains settings specific to your AWS environment.

### `stubs/diego-windows/iaas-settings.yml`

This stub is used during Diego Windows Cells manifest generation.
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

##### Enable Volume Services (experimental) (optional)

If you wish to enable volume services add the following property to the `cc` section of `$DEPLOYMENT_DIR/stubs/cf/properties.yml`:
```
cc:
  ...
  volume_services_enabled: true
```

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

## Setup a SQL database for Diego

These instructions configure Diego to use a relational database as the backing data store using one of the following methods:

* [Setting up an RDS MySQL](#setup-aws-rds-mysql)
* [Setting up an RDS PostgreSQL](#setup-aws-rds-postgresql)
* [Deploying a standalone CF-MySQL](#deploy-standalone-cf-mysql)
* [Using the PostgreSQL job from CF-Release](#use-the-postgresql-job-from-cf-release)

### Setup AWS RDS MySQL

The instructions below describe how to set up a *MariaDB* RDS instance that is
known to work with Diego.

1. From the AWS console homepage, click on `RDS` in the `Database` section.
1. Click on `Launch DB Instance` under Instances.
1. Click on the `MariaDB` tab and click the `Select` button.
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

### Setup AWS RDS PostgreSQL

To setup a PostgreSQL instance on RDS in AWS, follow the instructions above describing the setup of a MySQL AWS RDS instance, but select the `PostgreSQL` tab instead of the `MariaDB` tab in step 3.  Make sure to pick a version of PostgreSQL that is 9.4 or higher.

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

  0. Add the following `seeded_databases` property to configure a database for Diego to use. Replace `REPLACE_ME_WITH_DB_PASSWORD` with the desired password for the database:

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

### Use the PostgreSQL job from CF-Release

The PostgreSQL job in CF Release can be used as the database for Diego. Replace `REPLACE_ME_WITH_DB_PASSWORD` in your `stubs/cf/properties.yml` with your desired password, and [configure Diego to use this database](#use-of-granular-database-properties-for-mysql-or-postgresql).

```yaml
databases:
  ...
  roles:
    ...
    - name: diego:
      password: REPLACE_WITH_DB_PASSWORD
```

## Deploy Diego

### Fill in Diego-SQL stub

To configure Diego to communicate with the SQL instance, first create a Diego-SQL stub file at `$DEPLOYMENT_DIR/stubs/diego/diego-sql.yml` with the following contents:

```yaml
sql_overrides:
  bbs:
    db_driver: <driver>
    db_host: <sql-instance-endpoint>
    db_port: <port>
    db_username: diego
    db_password: <REPLACE_ME_WITH_DB_PASSWORD>
    db_schema: diego
    max_open_connections: 500
    require_ssl: null
    ca_cert: null
```

Fill in the bracketed parameters in the `db_driver`, `db_host`, `db_port` and `db_password` with the following values:

- `<driver>` could be either `mysql` or `postgres` depending on  the flavor of your backing data store.
- For AWS RDS:
  - The endpoint displayed at the top of the DB instance would replace `<sql-instance-endpoint>` in details page
  - `<port>` will take on the value of the port for the given DB instance.
- For Standalone CF-MySQL:
  - If configuring a Single Node CF-MySQL, `<sql-instance-endpoint>` would be the internal IP address and `<port>` would take on the port of the single MySQL node.
  - If configuring an Highly Available CF-MySQL with Consul use the consul service address (e.g. `mysql.service.cf.internal` for `<sql-instance-endpoint>` and `3306` for `<port>`).
- `<REPLACE_ME_WITH_DB_PASSWORD>`: The password chosen when you created the SQL instance.


#### SSL support

**Note:** The `sql_overrides.bbs.ca_cert` and `sql_overrides.bbs.require_ssl` properties should be provided only when deploying with an SSL-supported MySQL cluster. Set the `require_ssl` property to `true` to ensure that the BBS uses SSL to connect to the store, and set the `ca_cert` property to the contents of a certificate bundle containing the correct CA certificates to verify the certificate that the SQL server presents.

If enabling SSL for an RDS database, include the contents of `$DEPLOYMENT_DIR/certs/rds-combined-ca-bundle.pem` as the value of the `ca_cert` property:

```yaml
sql_overrides:
  bbs:
    ca_cert: |
      REPLACE_WITH_CONTENTS_OF_(DEPLOYMENT_DIR/certs/rds-combined-ca-bundle.pem)
```

### Verifying data migration

Follow steps in
[Migration of BBS Data from etcd to SQL](../../docs/data-stores.md#migration-of-bbs-data-from-etcd-to-sql)
to verify that the migration ran successfully.

## Create and Upload Volume Driver Release (experimental) (optional)

If you enabled volume services, create and upload your Driver's bosh release.

If you would like to use the `cephdriver` that we use for testing and development then you may use this [repo](https://github.com/cloudfoundry-incubator/cephfs-bosh-release).

## Deploying Diego

After deploying Cloud Foundry, you can now deploy Diego.

### Fill in the Property-Overrides Stub

To generate a manifest for the Diego deployment, replace the properties in the
`$DEPLOYMENT_DIR/stubs/diego/property-overrides.yml` file that are prefixed with `REPLACE_WITH_`.

Here is a summary of the properties that must be changed:

- Replace all instances of `REPLACE_WITH_ACTIVE_KEY_LABEL` with the desired key name (for example, `key-a`).
- Replace `REPLACE_WITH_A_SECURE_PASSPHRASE` with a unique passphrase associated with the active key label.

Component log levels and other deployment properties may also be overridden in this stub file.

This stub file also contains the contents of the BBS, and SSH-Proxy
certificates and keys generated above. If those files are regenerated, the
`deploy_aws_environment` script will update the property-overrides stub with
their new contents.

### Edit the Instance-Count-Overrides Stub

Copy the example stub to `$DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml`:

```bash
cp $DIEGO_RELEASE_DIR/examples/aws/stubs/diego/instance-count-overrides-example.yml $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml
```

Edit that file to change the instance counts of the deployed Diego VMs.

### Edit the Release-Versions Stub

Copy the example release-versions stub to the correct location:

```bash
cp $DIEGO_RELEASE_DIR/examples/aws/stubs/diego/release-versions.yml $DEPLOYMENT_DIR/stubs/diego/release-versions.yml
```

Edit it to fix the versions of the diego, garden-runc, and cflinuxfs2 releases in
the Diego deployment, instead of using the latest versions uploaded to the BOSH
director.

For example, to use version 1.0.0 of garden-runc-release, edit the stub to read:

```yaml
release-versions:
  diego: latest
  cflinuxfs2-rootfs: latest
  garden-runc: 1.0.0
```

### Fill in `Drivers` Stub (experimental) (optional)

If you enabled volume services, follow these directions to [fill in the drivers Stub](OPTIONAL.md#fill-in-drivers-stub) with your driver configuration.

### Generate the Diego manifest

See the full [manifest generation documentation](https://github.com/cloudfoundry/diego-release/docs/manifest-generation.md) for more generation instructions.
Remember that the `-n` instance-count-overrides flag and the `-v` release-versions flags are optional. If using a non-standard deployment (SQL, Volume Drivers, etc) follow the [generate the Diego manifest optional instructions](OPTIONAL.md#generate-the-diego-manifest).

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

#### Remove Colocated Diego etcd [optional]

The manifest generated above leaves etcd colocated with the BBS servers on the Diego `database_zN` instances, so that existing data in etcd can be migrated to the SQL database. Once that data is [migrated successfully](../../docs/data-stores.md#migration-of-bbs-data-from-etcd-to-sql), though, there is no reason to keep etcd in the deployment, and you may specify the `-x` flag on the manifest-generation script to remove it:

```bash
cd $DIEGO_RELEASE_DIR
./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -n $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego/release-versions.yml \
  -s $DEPLOYMENT_DIR/stubs/diego/diego-sql.yml \
  -x \
  > $DEPLOYMENT_DIR/deployments/diego.yml
```


### Upload garden-runc and cflinuxfs2-rootfs releases

1. Upload the latest garden-runc-release:
    ```bash
    bosh upload release https://bosh.io/d/github.com/cloudfoundry/garden-runc-release
    ```

    To upload a specific version of garden-runc-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry/garden-runc-release).

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

### Deploy Diego Windows Cells (optional)

To deploy a set of Diego cells using [garden-windows](https://github.com/cloudfoundry-incubator/garden-windows-bosh-release),
follow the directions in [Setup Garden Windows for Diego](OPTIONAL.md#setup-garden-windows-for-diego).
