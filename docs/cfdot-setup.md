# Setting up the Cfdot CLI tool

1. `bosh ssh` into a Diego deployment vm
1. Run `source /var/vcap/jobs/cfdot/bin/setup` to add the `cfdot` command to your path as well as setting default environment variables for BBS certs

See the [Cfdot documentation](https://github.com/cloudfoundry/cfdot) for more information on how to use the tool or run `cfdot -h` to show the help command.
