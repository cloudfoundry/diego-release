//go:build tools
// +build tools

package tools

import (
	_ "code.cloudfoundry.org/bbs/cmd/bbs"
	_ "code.cloudfoundry.org/cnbapplifecycle/cmd/builder"
	_ "code.cloudfoundry.org/cnbapplifecycle/cmd/launcher"
	_ "code.cloudfoundry.org/locket/cmd/locket"
	_ "code.cloudfoundry.org/routing-api/cmd/routing-api"
	_ "github.com/gogo/protobuf/protoc-gen-gogoslick"
	_ "github.com/nats-io/nats-server/v2"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
)

// This file imports packages that are used when running go generate, or used
// during the development process but not otherwise depended on by built code.
