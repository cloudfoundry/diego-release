// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelhttp // import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// Version is the current release version of the otelhttp instrumentation.
func Version() string {
<<<<<<< HEAD
<<<<<<< HEAD
	return "0.59.0"
=======
	return "0.54.0"
>>>>>>> 25a4f88bf (Update go.mod dependencies)
=======
	return "0.55.0"
>>>>>>> 58a961646 (Update go.mod dependencies)
	// This string is updated by the pre_release.sh script during release
}

// SemVersion is the semantic version to be supplied to tracer/meter creation.
//
// Deprecated: Use [Version] instead.
func SemVersion() string {
	return Version()
}
