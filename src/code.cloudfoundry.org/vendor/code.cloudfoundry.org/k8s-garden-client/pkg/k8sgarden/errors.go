package k8sgarden

import "errors"

// ErrNotSupported is returned by garden.Client and garden.Container methods
// that the Kubernetes-backed implementation does not support.
var ErrNotSupported = errors.New("not supported by the kubernetes-backed garden client")
