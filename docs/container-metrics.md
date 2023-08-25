# Container Metrics

A list of container metrics emitted by Diego. Each metric is a separate value in an envelope. Some metrics are separated into different envelopes to ensure Loggregator v1 subscribers can still receive these metrics.

| Metric                 | Description                                                                                                                                                | Type    | Unit                 |
| ---------------------- |------------------------------------------------------------------------------------------------------------------------------------------------------------|---------| -------------------- |
| `absolute_entitlement` | Length of time the container is entitled to spend using CPU.                                                                                               | Gauge   | nanoseconds          |
| `absolute_usage`       | Total length of time container has spent using CPU.                                                                                                        | Gauge   | nanoseconds          |
| `container_age`        | Length of time container has existed for.                                                                                                                  | Gauge   | nanoseconds          |
| `cpu`                  | Percentage of time container spent using CPU.                                                                                                              | Gauge   | percentage           |
| `disk`                 | Disk space in use by this container.                                                                                                                       | Gauge   | bytes                |
| `disk_quota`           | User requested disk quota set on the DesiredLRP for this container.                                                                                        | Gauge   | bytes                |
| `memory`               | Memory in use by this container. If the per-instance proxy is enabled, memory usage is scaled set based on the additional memory allocation for the proxy. | Gauge   | bytes                |
| `memory_quota`         | User requested memory quota set on the DesiredLRP for this container.                                                                                      | Gauge   | bytes                |
| `spike_start`          | Time at which a spike over a containers CPU entitlement started.                                                                                           | Gauge   | unix epoch timestamp |
| `spike_end`            | Time at which a spike over a container's CPU entitlement ended.                                                                                            | Gauge   | unix epoch timestamp |
| `rx_bytes`             | Received network traffic.                                                                                                                                  | Counter | bytes                |
| `tx_bytes`             | Transmitted network traffic.                                                                                                                               | Counter | bytes                |
