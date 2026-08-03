package locket

import (
	"path"
	"time"
)

const DefaultSessionTTL = 15 * time.Second
const DefaultSessionTTLInSeconds int64 = 15
const MonitorRetryTime = 2 * time.Second
const RetryInterval = 5 * time.Second
const SQLRetryInterval = time.Second
const ExpirationMetricsInterval = 60 * time.Second
const DBHealthCheckTimeout = 5 * time.Second
const DBHealthCheckInterval = 10 * time.Second
const DBHealthCheckFailureThreshold = 3

const LockSchemaRoot = "v1/locks"

func LockSchemaPath(lockName ...string) string {
	return path.Join(LockSchemaRoot, path.Join(lockName...))
}
