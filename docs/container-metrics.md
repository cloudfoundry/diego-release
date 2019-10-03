# Container Metrics

A list of container metrics emitted by Diego. Each metric is a separate value on a Gauge envelope. Some metrics are separated into different envelopes to ensure Loggregator v1 subscribers can still receive these metrics.

| Metric                 | Description                                                                                                                                                | Unit                 |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------- |
| `absolute_entitlement` | Length of time the container is entitled to spend using CPU.                                                                                               | nanoseconds          |
| `absolute_usage`       | Total length of time container has spent using CPU.                                                                                                        | nanoseconds          |
| `container_age`        | Length of time container has existed for.                                                                                                                  | nanoseconds          |
| `cpu`                  | Percentage of time container spent using CPU.                                                                                                              | percentage           |
| `disk`                 | Disk space in use by this container.                                                                                                                       | bytes                |
| `disk_quota`           | User requested disk quota set on the DesiredLRP for this container.                                                                                        | bytes                |
| `memory`               | Memory in use by this container. If the per-instance proxy is enabled, memory usage is scaled set based on the additional memory allocation for the proxy. | bytes                |
| `memory_quota`         | User requested memory quota set on the DesiredLRP for this container.                                                                                      | bytes                |
| `spike_start`          | Time at which a spike over a containers CPU entitlement started.                                                                                           | unix epoch timestamp |
| `spike_end`            | Time at which a spike over a container's CPU entitlement ended.                                                                                            | unix epoch timestamp |
