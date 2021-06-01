// +build tools

package tools

import (
	_ "github.com/nats-io/nats-server"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "github.com/hashicorp/consul"
)

// This file imports packages that are used when running go generate, or used
// during the development process but not otherwise depended on by built code.
