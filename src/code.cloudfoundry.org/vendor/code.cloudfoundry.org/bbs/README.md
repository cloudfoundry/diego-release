# BBS Server [![GoDoc](https://godoc.org/github.com/cloudfoundry/bbs?status.svg)](https://godoc.org/github.com/cloudfoundry/bbs)

**Note**: This repository should be imported as `code.cloudfoundry.org/bbs`.

API to access the database for Diego.

A general overview of the BBS is documented [here](doc).

## Reporting issues and requesting features

Please report all issues and feature requests in [cloudfoundry/diego-release](https://github.com/cloudfoundry/diego-release/issues).

## API

To interact with the BBS from outside of Diego, use the methods provided on the
[`Client` interface](https://godoc.org/github.com/cloudfoundry/bbs#Client).

Components within Diego may use the full [`InternalClient`
interface](https://godoc.org/github.com/cloudfoundry/bbs#InternalClient) to modify internal state.

## Code Generation

The protobuf models in this repository require version 3.5 or later of the `protoc` compiler.

### OSX

On Mac OS X with [Homebrew](http://brew.sh/), run the following to install it:

```
brew install protobuf
```

### Linux

1. Download a zip archive of the latest protobuf release from [here](https://github.com/google/protobuf/releases).
1. Unzip the archive in `/usr/local` (including /bin and /include folders).
1. `chmod a+x /usr/local/bin/protoc` to make sure you can use the binary.

> If you already have an older version of protobuf installed, you must
> uninstall it first by running `brew uninstall protobuf`

Install the `gogoproto` compiler by running:

```
go install github.com/gogo/protobuf/protoc-gen-gogoslick
```

Run `go generate ./...` from the root directory of this repository to generate code from the `.proto` files as well as to generate fake implementations of certain interfaces for use in test code.

### Generating ruby models for BBS models

The following documentation assume the following versions:

1. [protoc](https://github.com/google/protobuf/releases) `> v3.5.0`
2. [ruby protobuf gem](https://github.com/ruby-protobuf/protobuf) `> 3.6.12`

Run the following commands from the `models` directory to generate `.pb.rb`
files for the BBS models:

1. `sed -i'' -e 's/package models/package diego.bbs.models/' ./*.proto`
1. `protoc -I../../vendor --proto_path=. --ruby_out=/path/to/ruby/files *.proto`

**Note** Replace `/path/to/ruby/files` with the desired destination of the
`.pb.rb` files. That directory must exist before running this command.

**Note** The above steps assume that
`github.com/gogo/protobuf/gogoproto/gogo.proto` is on the `GOPATH`.

## SQL

See the instructions in [Running the SQL Unit Tests](https://github.com/cloudfoundry/diego-release/blob/develop/CONTRIBUTING.md#running-the-sql-unit-tests)
for testing against a SQL backend

See [Migrations](https://github.com/cloudfoundry/bbs/blob/master/doc/bbs-migration.md) for information about writing database migrations.

## Run Tests

1. First setup your [GOPATH and install the necessary dependencies](https://github.com/cloudfoundry/diego-release/blob/develop/CONTRIBUTING.md#initial-setup) for running tests.
1. Setup a MySQL server or a postgres server. [Please follow these instructions.](https://github.com/cloudfoundry/diego-release/blob/develop/CONTRIBUTING.md#running-the-sql-unit-tests)
1. Run the tests from the root directory of the bbs repo:
```
SQL_FLAVOR=mysql ginkgo -r -p -race
```
