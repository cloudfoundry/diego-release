# Deploying Diego to AWS

## Table of Contents
1. [Prerequisites](#set-up-dependencies)
1. [Initializing AWS for Cloud Foundry](#creating-the-aws-environment)
1. [Deploying CF](#deploying-cloud-foundry)
1. [Deploying Diego](#deploying-diego)

## Set Up Dependencies

### Setting Up Local Environment

As part of our deployment process, you must install the following dependencies:

* [Go 1.4.3](https://golang.org/doc/install)
* [godep](https://github.com/tools/godep)
* [boosh](https://github.com/vito/boosh)
* [spiff](https://github.com/cloudfoundry-incubator/spiff)
* [aws cli](https://aws.amazon.com/cli/)
* [jq](https://stedolan.github.io/jq/)
* [ruby](https://www.ruby-lang.org/en/documentation/installation/)
* [bosh cli](http://bosh.io/docs/bosh-cli.html)
* [bosh init](https://bosh.io/docs/install-bosh-init.html)

You must also clone the following github repositories:

* [cf-release](https://github.com/cloudfoundry/cf-release)
* [diego-release](https://github.com/cloudfoundry-incubator/diego-release)

### Deployment Directory

Our deployment process requires that you create a directory for each deployment
which will hold the necessary configuration to deploy bosh, cf-release, and
diego-release.
This directory will be referred to as `$DEPLOYMENT_DIR` later in these instructions.

### AWS Requirements

Before deploying the bosh director, you must create the following resources in
your AWS account through the AWS console:

* IAM User Policy
  1. From the AWS console homepage, click on `Identity & Access Management`
  2. Click on the `Policies` link
  3. Click on the `Create Policy` button
  4. Select `Create Your Own Policy`
  5. Enter `bosh-aws-policy` as the `Policy Name`
  6. Enter:
  ```json
  {
    "Version": "1",
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
          "vpc:*"
          "elasticloadbalancing:*",
          "route53:*"
        ],
        "Resource": "*"
      }
    ]
  }
  ```

* IAM User
  1. From the AWS console homepage, click on `Identity & Access Management`
  2. Click on `Users` link
  3. Click on the `Create New Users` button
  4. Fill in only one user name
  5. Make sure that the `Generate an access key for each user` checkbox is checked and click `Create`
  6. Click `Download Credentials` at the bottom of the screen
  7. Click  the `Cancel` link to return to the IAM Users page
  8. Select the user that you created
  9. Click the `Attach Policy` button
  10. Filter for `bosh-aws-policy` in the filter box
  11. Select `bosh-aws-policy` and click the `Attach Policy` button

* AWS keypair for your bosh director
  1.  From your AWS EC2 page click on the `Key Pairs` link
  2.  Click the `Create Key Pair` button at the top of the page
  3.  When prompted for the key name, enter `bosh`
  4.  Move the downloaded `bosh.pem` key to `$DEPLOYMENT_DIR/keypair/` and rename the key to `id_rsa_bosh`

* Route 53 Hosted Zone
  1.  From the aws console homepage, click on `Route 53`
  2.  Select `Hosted zones` from the left sidebar
  3.  Click the `Create Hosted Zone` button
  4.  Fill in the domain name for your Cloud Foundry deployment

  By default, the domain name for your hosted zone will be the root domain of all apps deployed to your cloud foundry instance.

  eg:
   ```
   domain = foo.bar.com
   app name = `hello-world`. This will create a default route of hello-world.domain

   http://hello-world.foo.bar.com will be the root url address of your application
   ```

### Deployment Directory Setup

After creating the necessary resources in AWS, you must populate the
`DEPLOYMENT_DIR` in the following format. Each of the files is explained further
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
|   |-(releases.yml)
|   |-(users.yml)
|   |-(stemcell.yml)
```

#### bootstrap_environment

This script exports your AWS default region and access/secret keys as environment variables.
The `AWS_ACCESS_KEY_ID` key must match the AWS IAM user's access key id and the `AWS_SECRET_ACCESS_KEY`
is the private key generated during the [IAM user creation](#aws-requirements).

eg:
```
export AWS_DEFAULT_REGION=us-east-1
export AWS_ACCESS_KEY_ID=xxxxxxxxxxxxxxxxxxx
export AWS_SECRET_ACCESS_KEY='xxxxxxxxxxxxxxxxxxxxxx'
```

#### keypair/id_rsa_bosh

This is the private key pair generated for the BOSH director when the [AWS keypair](#aws-requirements) was created.

#### certs/elb-cfrouter.key && certs/elb-cfrouter.pem

An SSL certificate for the domain where Cloud Foundry will be accessible is required. If you do not already provide a certificate,
you can generate a self signed cert following the commands below:

```
openssl genrsa -out elb-cfrouter.key 2048
openssl req -new -key elb-cfrouter.key -out elb-cfrouter.csr # Enter `*.YOUR_CF_DOMAIN` as the "Common Name"
```

You can leave all of the requested inputs blank, except for `Common Name` which should be `*.YOUR_CF_DOMAIN`. Then run:

```
openssl x509 -req -in elb-cfrouter.csr -signkey elb-cfrouter.key -out elb-cfrouter.pem
```

#### stubs/domain.yml

The `domain.yml` should be assigned to the domain that was generated when the [route 53 hosted zone](#aws-requirements) was created.

eg:
```yaml
---
domain: <your-domain.com>
```

#### stubs/infrastructure/availability_zones.yml

This yaml file defines the 3 zones that will host your Cloud Foundry Deployment.

eg:
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
Value (us-east-1b) for parameter availabilityZone is invalid Subnets can currently only be created in the following availability zones: us-east-1d, us-east-1b, us-east-1a, us-east-1e
```

you will need to update this file with acceptable availability zone values.

#### stubs/bosh-init/releases.yml

To deploy the bosh director, bosh-init's `releases.yml` must specify `bosh` and `bosh-aws-cpi` releases by `url` and `sha1`.

eg:
```yaml
---
releases:
  - name: bosh
    url: URL_TO_LATEST_BOSH_BOSH_RELEASE
    sha1: SHA1_OF_LATEST_BOSH_BOSH_RELEASE
  - name: bosh-aws-cpi
    url: URL_TO_LATEST_BOSH_AWS_CPI_BOSH_RELEASE
    sha1: SHA1_OF_LATEST_BOSH_AWS_CPI_BOSH_RELEASE
```

Releases for `bosh` can be found [here](http://bosh.io/releases/github.com/cloudfoundry/bosh?all=1).
Releases for `bosh-aws-cpi` can be found [here](http://bosh.io/releases/github.com/cloudfoundry-incubator/bosh-aws-cpi-release?all=1).

#### stubs/bosh-init/users.yml

This file defines the admin users for your bosh director.

eg:
```yaml
---
BoshInitUsers:
  - {name: admin, password: YOUR_PASSWORD}
```

#### stubs/bosh-init/stemcell.yml

This file defines which stemcell to use on the bosh director. Stemcells can be found
[here](http://bosh.io/stemcells/bosh-aws-xen-ubuntu-trusty-go_agent), and must be specified by their `url` and `sha1`.

eg:
```yaml
---
BoshInitStemcell:
  url: https://bosh.io/d/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent?v=3091
  sha1: 21ce6eb039179bb5b1706adfea4c161ea20dea1f
```

Currently bosh.io does not provide the sha1 of stemcells. You must download the
stemcell locally and calcuate the sha1 manually. This can be done on OSX by running:

```
shasum /path/to/stemcell
```

### Adding Security

In order to properly secure your Cloud Foundry deployment, you must generate SSL certificates and keys to secure traffic between components.

We provide two scripts to help generate the necessary SSL certificates:

1. consul cert generation
```
cd $DEPLOYMENT_DIR/certs
$CF_RELEASE_DIRECTORY/scripts/generate-consul-certs
```
1. diego cert generation
```
$DIEGO_RELEASE_DIRECTORY/scripts/generate-diego-certs
mv $DIEGO_RELEASE_DIR/diego-certs/* $DEPLOYMENT_DIR/certs
```

After running theses scripts, you should see the following output:
```
DEPLOYMENT_DIR
|-certs
|  |-consul-certs # generated via cf-release/scripts/generate-consul-certs
|  |  |- agent.crt
|  |  |- agent.key
|  |  |- server-ca.crt
|  |  |- server-ca.key
|  |  |- server.crt
|  |  |- server.key
|  |-etcd-certs # generated via cf-release/scripts/generate-diego-certs
|  |  |- client.crt
|  |  |- client.key
|  |  |- server.crt
|  |  |- server.key
|  |  |- peer.crt
|  |  |- peer.key
|  |-bbs-certs # generated via cf-release/scripts/generate-diego-certs
|  |  |- client.crt
|  |  |- client.key
|  |  |- server.crt
|  |  |- server.key
|  |- diego-ca.crt
|  |- diego-ca.key
|  |- etcd-peer-ca.crt
|  |- etcd-peer-ca.key
|-keypair
| |-(ssh-proxy-hostkey.pem)
| |-(ssh-proxy-hostkey.pem.pub)
| |-(ssh-proxy-hostkey-fingerprint)
| |-(uaa)
| |-(uaa.pem.pub)
```

####<a name="generating-ssh-proxy-host-key"></a>Generating SSH Proxy Host Key and Fingerprint

In order for SSH to work for diego-release, you must generate the SSH Proxy host key and fingerprint.
This can be done by running:

```
ssh-keygen -f $DEPLOYMENT_DIR/keypair/ssh-proxy-hostkey.pem
ssh-keygen -lf $DEPLOYMENT_DIR/keypair/ssh-proxy-hostkey.pem.pub -E md5 | cut -d ' ' -f2 | sed "s/MD5://" > $DEPLOYMENT_DIR/keypair/ssh-proxy-hostkey-fingerprint
```

The `ssh-proxy-host-key.pem` will contain the PEM encoded host key for the diego release manifest.

The md5 host key fingerprint needs to be added to the cf release manifest `cf.yml` under `properties.app_ssh.host_key_fingerprint` before you deploy cf release.

#### Generating UAA Private/Public Keys

In order to properly configure UAA, you need to generate an RSA keypair.
This can be done by running the following:

```
ssh-keygen -t rsa -b 4096 -f $DEPLOYMENT_DIR/keypair/uaa
openssl rsa -in $DEPLOYMENT_DIR/uaa -pubout > $DEPLOYMENT_DIR/uaa.pub
```

#### certs/consul

These generated certificates are used to set SSL properties for the consul VMs.
By default, these properties will be set in your `stubs/cf/properties.yml`.
For more information on how to configure SSL for consul, please see [these instructions](http://docs.cloudfoundry.org/deploying/common/consul-security.html).

#### certs/etcd and certs/bbs

These generated certificates are used to configure SSL between components in Diego.
This ensures that communication with the database is secure and encrypted.

## Creating the AWS environment

To create the AWS environment and two VMs essential to the Cloud Foundry infrastructure,
you need to run `./deploy_aws_environment create CF_RELEASE_DIRECTORY DEPLOYMENT_DIR` **from this repository**.
This may take up to 30 minutes.

The `./deploy_aws_environment` script has three possible actions.
  * `create` spins up an AWS Cloud Formation Stack based off of the stubs filled out above
  * run `update` if you change your stubs under `DEPLOYMENT_DIR/stubs/infrastructure` or there was an update to this repository
  * `skip` will upgrade your bosh director, but will not touch the AWS environment

The second parameter is the **absolute path** to CF_RELEASE_DIRECTORY.

The third parameter is your `DEPLOYMENT_DIR` and must be structured as defined above. The deployment process
generates additional stubs that include the line "GENERATED: NO TOUCHING".

The generated stubs are:
```
DEPLOYMENT_DIR
|-stubs
| |-(director-uuid.yml) # the bosh director's unique id
| |-(aws-resources.yml)  # general metadata about our cloudformation deployment
| |-cf
| | |-(stub.yml) # networks, zones, s3 buckets for our Cloud Foundry deployment
| | |-(properties.yml) # consul configuration, shared secrets
| | |-(domain.yml) # networks, zones, s3 buckets for our Cloud Foundry deployment
| |-diego
| | |-(proprety-overrides.yml)
| | |-(iaas-settings.yml)
| |-infrastructure
|   |-(certificates.yml) # for our aws-provided elb
|   |-(cloudformation.json) # aws' deployed cloudformation.json
|-deployments
| |-bosh-init
|   |-(bosh-init.yml) # bosh director deployment
```

### stubs/cf/stub.yml

As part of our deploy_aws_environment script we generate a partial stub for your
Cloud Foundry deployment. It is a generated stub that contains AWS specific information.
This stub should not be edited manually.

### stubs/cf/properties.yml

As part of our deploy_aws_environment script we copy a partial stub for your
Cloud Foundry deployment. This stub is discussed in more detail in the
[generate manifest](#manifest-generation) section.

### stubs/diego/property-overrides.yml

This stub will be used as part of Diego manifest generation and was constructed from
your deployed AWS infrastructure, as well as our default template. This stub proveds
the skeleton for our certs generated in the [Prerequisites](#adding-security) section,
as well as setting components log level.

### stubs/diego/iaas-settings.yml

This stub will be used as part of Diego manifest generation.
It defines the infastructure specifc settings defined by your AWS environment.

## Route53 for BOSH Director (optional)

If you want your BOSH director to be accessible using the [Route53 hosted zone](#aws-requirements) earlier,
you need to perform the following steps:

  1. Obtain the public IP address of the BOSH director in the EC2 dashboard
  1. Click on the `Route53` link on the AWS console
  1. Click the `Hosted Zones` link
  1. Click on the hosted zone created earlier
  1. Click the `Create Record Set` button
  1. Enter `bosh` for the `Name`.
  1. Enter the public IP address of the bosh director for the value
  1. Click the `Create` button

## Deploying Cloud Foundry

### Manifest Generation

To deploy Cloud Foundry, you need a stub similar to the one from the [Cloud Foundry Documentation](http://docs.cloudfoundry.org/deploying/aws/cf-stub.html).
The generated stub `DEPLOYMENT_DIR/stubs/cf/stub.yml` already has a number of these properties filled out for you.
The provided stub `DEPLOYMENT_DIR/stubs/cf/properties.yml` has some additional properties that need to be specified.
For more information on stubs for cf-release manifest generation, please refer to the documentation [here](http://docs.cloudfoundry.org/deploying/aws/cf-stub.html#editing).

#### Fill in Properties Stub

In order to correctly generate a manifest for the cf-release deployment, you must
replace certain values in the provided `$DEPLOYMENT_DIR/stubs/cf/properties.yml`.
Every value that needs to be replaced is prefixed with `REPLACE_ME_WITH`.

** Note: ** If you did not generate a self signed certificate for the [elb-cfrouter.pem](#deployment-directory-setup)
and are using a certificate signed by a trusted certificate authority, you will need to change the value of `properties.ssl.skip_cert_verify` from `true` to `false`.

#### Diego Stub

Cloud Foundry Documention manifest generation doesn't create some VMs and properties that Diego depends on.
It also includes some unnecessary VMs and properties that Diego doesn't need. To correct this, the provided cf stub `./stubs/cf/diego.yml` is used when generating the Cloud Foundry manifest.

#### Generate

After following the instructions to generate a `DEPLOYMENT_DIR/stubs/cf/stub.yml` stub and downloading the cf-release directory, run
the following command **inside this repository** to generate the Cloud Foundry manifest:

```
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

```
bosh target bosh.YOUR_CF_DOMAIN
```

When prompted for the username and password, they are the credentials set in the `DEPLOYMENT_DIR/stubs/bosh-init/users.yml` stub.

### Upload the BOSH Stemcell

Upload the lastest BOSH stemcell for AWS to the bosh director.
You can find the latest stemcell [here](http://bosh.io/stemcells/bosh-aws-xen-hvm-ubuntu-trusty-go_agent).

```
bosh upload stemcell /path/to/stemcell
```

### Create and Upload the CF Release

In order to deploy CF Release, you must create and upload the release to the director using the following commands:

```
cd $CF_RELEASE_DIR
bosh --parallel 10 create release
bosh upload release
```

### Deploy

Set the deployment manifest and deploy with the following commands:

```
bosh deployment $DEPLOYMENT_DIR/deployments/cf.yml
bosh deploy
```

From here, follow the documentation on [deploying a Cloud Foundry with BOSH](http://docs.cloudfoundry.org/deploying/common/deploy.html). Note that the deployment
can take up to 30 minutes.

## Deploying Diego

After deploying Cloud Foundry, you are now able to begin deploying Diego.
`$DEPLOYMENT_DIR` must be set to the absoulte path to your local deployment directory.

**NOTE: All the following commands should be run from the root of this repository.**

### Fill in Generated Property Overrides Stub

In order to generate a manifest for diego-release, you need to replace certain values in the provided `$DEPLOYMENT_DIR/stubs/diego/property-overrides.yml`.
Every property that needs to be replaced is prefixed with `REPLACE_ME_WITH_`.

Here is a summary of the properties that need to be changed:
  * REPLACE_ME_WITH_ACTIVE_KEY_LABEL: any desired key name
  * REPLACE_ME_WITH_A_SECURE_PASSPHRASE: a unique passphrase associated with the active key label
  * ETCD and BBS certs: if you need to generate them, [see these instructions](#adding-security)
  * SSH Proxy Host Key: this is the [key generated](#generating-ssh-proxy-host-key) earlier in these docs

### (Optional) Edit instance-count-overrides stub

Copy the example to the correct location.
```
cp examples/aws/diego/stubs/instance-count-overrides-example.yml $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml
```
Edit it if you want to change the number of instances of each of the jobs to create.

### (Optional) Edit release-versions stub

Copy the example to the correct location.
```
cp examples/aws/diego/stubs/release-versions.yml $DEPLOYMENT_DIR/stubs/diego/release-versions.yml
```

If you want to edit it, the format is:
```yml
release-versions:
  - diego: latest
  - etcd: 22
  - garden-linux: 331
```

### Generate the diego manifest

Remember that the last two arguments for `instance-count-overrides` and `release-versions`
are optional.
```
./scripts/generate-deployment-manifest \
  -c $DEPLOYMENT_DIR/deployments/cf.yml \
  -i $DEPLOYMENT_DIR/stubs/diego/iaas-settings.yml \
  -p $DEPLOYMENT_DIR/stubs/diego/property-overrides.yml \
  -n $DEPLOYMENT_DIR/stubs/diego/instance-count-overrides.yml \
  -v $DEPLOYMENT_DIR/stubs/diego/release-versions.yml \
  > $DEPLOYMENT_DIR/deployments/diego.yml
```

### Upload Garden and ETCD release
1. Upload the latest garden-linux-release:
    ```
    bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/garden-linux-release
    ```

    If you wish to upload a specific version of garden-linux-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry-incubator/garden-linux-release).

1. Upload the latest etcd-release:
    ```
    bosh upload release https://bosh.io/d/github.com/cloudfoundry-incubator/etcd-release
    ```

    If you wish to upload a specific version of etcd-release, or to download the release locally before uploading it, please consult directions at [bosh.io](http://bosh.io/releases/github.com/cloudfoundry-incubator/etcd-release).

### Deploy Diego

These commands may take up to an hour. Be patient; it's worth it.
```
bosh deployment $DEPLOYMENT_DIR/deployments/diego.yml
bosh --parallel 10 create release --force
bosh upload release
bosh deploy
```
