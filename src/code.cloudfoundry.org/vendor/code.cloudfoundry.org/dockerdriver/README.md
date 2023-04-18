# dockerdriver

This repo contains a server/client skeleton and the interfaces to to implement a docker volume driver server for use with Diego's [volume manager](https://github.com/cloudfoundry-incubator/volman).

## Reporting issues and requesting features

Please report all issues and feature requests in [cloudfoundry/diego-release](https://github.com/cloudfoundry/diego-release/issues).

## Development
- To run the tests, run `go run github.com/onsi/ginkgo/v2/ginkgo -r`, `go test`, or `ginkgo -r` if you have [Ginkgo](https://github.com/onsi/ginkgo) installed.
- To re-generate the test fakes, run `go generate`.