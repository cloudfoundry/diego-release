//go:build tools
// +build tools

package tools

import (
	_ "github.com/gogo/protobuf/protoc-gen-gogoslick"
	_ "github.com/nats-io/nats-server/v2"
	_ "github.com/onsi/ginkgo/ginkgo"
)

// This file imports packages that are used when running go generate, or used
// during the development process but not otherwise depended on by built code.
