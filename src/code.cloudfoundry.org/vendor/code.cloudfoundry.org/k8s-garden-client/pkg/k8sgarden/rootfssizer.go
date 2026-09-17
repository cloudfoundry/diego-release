package k8sgarden

// ZeroRootFSSizer reports a rootfs size of 0 for any path.
//
// The Kubernetes-backed garden client already accounts for the rootfs size
// directly (Create subtracts it from the container's ephemeral-storage limit
// and BulkMetrics folds it into DiskStat.TotalBytesUsed). When the executor
// uses this client it must therefore be configured with a RootFSSizer that
// returns 0, so it does not subtract the rootfs size a second time. Wire this
// type into the executor's configuration for that purpose.
type ZeroRootFSSizer struct{}

func (ZeroRootFSSizer) RootFSSizeFromPath(string) uint64 { return 0 }
