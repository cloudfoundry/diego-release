# Setting up the `cfdot` CLI tool

The BOSH release for Diego contains a `cfdot` job template that deploys `cfdot` and `jq` binaries as well as a `setup` script to make them easy to invoke. If you use the [`generate-deployment-manifest`](../scripts/generate-deployment-manifest) script to generate your Diego manifest, `cfdot` is already available on the BOSH-deployed Diego VMs. To use it:

1. Run `bosh ssh <DIEGO_JOB>/<INDEX>` to start an SSH session on a Diego deployment VM.

1. Run `source /var/vcap/jobs/cfdot/bin/setup` to add the `cfdot` and `jq` executables to your PATH as well as to set environment variables for communication to the BBS API server.

See the [`cfdot` documentation](https://github.com/cloudfoundry/cfdot) for more information on how to use the tool or run `cfdot --help` to show usage.
