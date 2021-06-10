// +build tools

package tools

import (
	_ "code.cloudfoundry.org/certsplitter/cmd/certsplitter"
	_ "github.com/gogo/protobuf/protoc-gen-gogoslick"
	_ "github.com/hashicorp/consul"
	_ "github.com/nats-io/nats-server"
	_ "github.com/onsi/ginkgo/ginkgo"
)

// This file imports packages that are used when running go generate, or used
// during the development process but not otherwise depended on by built code.
