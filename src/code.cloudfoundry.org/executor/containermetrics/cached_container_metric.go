package containermetrics

type CachedContainerMetrics struct {
	MetricGUID       string  `json:"metric_guid"`
	CPUUsageFraction float64 `json:"cpu_usage_fraction"`
	DiskUsageBytes   uint64  `json:"disk_usage_bytes"`
	DiskQuotaBytes   uint64  `json:"disk_quota_bytes"`
	MemoryUsageBytes uint64  `json:"memory_usage_bytes"`
	MemoryQuotaBytes uint64  `json:"memory_quota_bytes"`
	RxBytes          *uint64 `json:"rx_bytes,omitempty"`
	TxBytes          *uint64 `json:"tx_bytes,omitempty"`
}
