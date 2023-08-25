# Container Metrics

A list of container metrics emitted by Diego. Each metric is a separate value in an envelope. Some metrics are separated into different envelopes to ensure Loggregator v1 subscribers can still receive these metrics.

| Metric                 | Type    | Description                                                                                                                                                | Unit                 |
| ---------------------- |---------| ---------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------- |
| `absolute_entitlement` | Gauge   | Length of time the container is entitled to spend using CPU.                                                                                               | nanoseconds          |
| `absolute_usage`       | Gauge   | Total length of time container has spent using CPU.                                                                                                        | nanoseconds          |
| `container_age`        | Gauge   | Length of time container has existed for.                                                                                                                  | nanoseconds          |
| `cpu`                  | Gauge   | Percentage of time container spent using CPU.                                                                                                              | percentage           |
| `rx_bytes`             | Counter | Bytes received by the container.                                                                                                                           | bytes                |
| `tx_bytes`             | Counter | Bytes transmitted by the container.                                                                                                                        | bytes                |
| `disk`                 | Gauge   | Disk space in use by this container.                                                                                                                       | bytes                |
| `disk_quota`           | Gauge   | User requested disk quota set on the DesiredLRP for this container.                                                                                        | bytes                |
| `memory`               | Gauge   | Memory in use by this container. If the per-instance proxy is enabled, memory usage is scaled set based on the additional memory allocation for the proxy. | bytes                |
| `memory_quota`         | Gauge   | User requested memory quota set on the DesiredLRP for this container.                                                                                      | bytes                |
| `spike_start`          | Gauge   | Time at which a spike over a containers CPU entitlement started.                                                                                           | unix epoch timestamp |
| `spike_end`            | Gauge   | Time at which a spike over a container's CPU entitlement ended.                                                                                            | unix epoch timestamp |
