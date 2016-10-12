# Diego Metrics

* [Auctioneer](#auctioneer)
* [BBS](#bbs)
* [Converger](#converger)
* [Rep](#rep)
* [Route Emitter](#route-emitter)
* [SSH Proxy](#ssh-proxy)
* [General Golang metrics](#general-golang-metrics)

## Auctioneer

| Metric                                      | Description                                                                                                                                                | Unit             |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ----             |
| `AuctioneerFailedCellStateRequests`         | Cumulative number of cells the auctioneer failed to query for state. Emitted during each auction.                                                          | number           |
| `AuctioneerFetchStatesDuration`             | Time the auctioneer took to fetch state from all the cells when running its auction. Emitted during each auction.                                          | ns               |
| `AuctioneerLRPAuctionsFailed`               | Cumulative number of LRP instances that the auctioneer failed to place on Diego cells. Emitted during each auction.                                        | number           |
| `AuctioneerLRPAuctionsStarted`              | Cumulative number of LRP instances that the auctioneer successfully placed on Diego cells. Emitted during each auction.                                    | number           |
| `AuctioneerTaskAuctionsFailed`              | Cumulative number of Tasks that the auctioneer failed to place on Diego cells.  Emitted during each auction.                                               | number           |
| `AuctioneerTaskAuctionsStarted`             | Cumulative number of Tasks that the auctioneer successfully placed on Diego cells. Emitted during each auction.                                            | number           |
| `LockHeld.` `v1-locks-auctioneer_lock`         | Whether an auctioneeer holds the auctioneer lock: 1 means the lock is held, and 0 means the lock was lost. Emitted periodically by the active auctioneer.  | 0 or 1 (boolean) |
| `LockHeldDuration.` `v1-locks-auctioneer_lock` | Time the active auctioneeer has held the auctioneer lock. Emitted periodically by the active auctioneer.                                                   | ns               |
| `RequestCount`                              | Cumulative number of requests the auctioneer has handled through its API.  Emitted periodically.                                                           | number           |
| `RequestLatency`                            | Time the auctioneer took to handle requests to its API endpoints. Emitted when the auctioneer handles requests.                                            | ns               |

## BBS

| Metric                                                | Description                                                                                                                                                                                                                      | Unit              |
| -------------------------------------------           | ----------------------------------------------------------------------------------------------------------------------------------------------------------                                                                       | ----              |
| `BBSMasterElected`                                    | Emitted once when the BBS is elected as master.                                                                                                                                                                                  | number (always 1) |
| `ConvergenceLRPDuration`                              | Time the BBS took to run its LRP convergence pass. Emitted every time LRP convergence runs.                                                                                                                                      | ns                |
| `ConvergenceLRPPreProcessingActualLRPsDeleted`        | Cumulative number of times the BBS has detected and deleted a malformed ActualLRP in its LRP convergence pass. Emitted periodically.                                                                                             | number            |
| `ConvergenceLRPPreProcessingMalformedRunInfos`        | Cumulative number of times the BBS has detected a malformed DesiredLRP RunInfo in its LRP convergence pass. Emitted periodically.                                                                                                | number            |
| `ConvergenceLRPPreProcessingMalformedSchedulingInfos` | Cumulative number of times the BBS has detected a malformed DesiredLRP SchedulingInfo in its LRP convergence pass. Emitted periodically.                                                                                         | number            |
| `ConvergenceLRPPreProcessingOrphanedRunInfos`         | Cumulative number of times the BBS has detected and deleted an orphaned DesiredLRP RunInfo in its LRP convergence pass. Emitted periodically.                                                                                    | number            |
| `ConvergenceLRPRuns`                                  | Cumulative number of times BBS has run its LRP convergence pass. Emitted periodically.                                                                                                                                           | number            |
| `ConvergenceTaskDuration`                             | Time the BBS took to run its Task convergence pass. Emitted every time Task convergence runs.                                                                                                                                    | ns                |
| `ConvergenceTaskRuns`                                 | Cumulative number of times the BBS has run its Task convergence pass. Emitted periodically.                                                                                                                                      | number            |
| `ConvergenceTasksKicked`                              | Cumulative number of times the BBS has updated a Task during its Task convergence pass. Emitted periodically.                                                                                                                    | number            |
| `ConvergenceTasksPruned`                              | Cumulative number of times the BBS has deleted a malformed Task during its Task convergence pass. Emitted periodically.                                                                                                          | number            |
| `CrashedActualLRPs`                                   | Total number of LRP instances that have crashed. Emitted periodically.                                                                                                                                                           | number            |
| `CrashingDesiredLRPs`                                 | Total number of DesiredLRPs that have at least one crashed instance. Emitted periodically.                                                                                                                                       | number            |
| `Domain.` `<domain-name>`                             | Whether the `<domain-name>` domain is up-to-date, so that instances from that domain have been synchronized with DesiredLRPs for Diego to run. 1 means the domain is up-to-date, no data means it is not. Emitted periodically. | 0 or 1 (boolean)  |
| `ETCDLeader`                                          | Index of the leader node in the etcd cluster. Emitted periodically.                                                                                                                                                              | number            |
| `ETCDRaftTerm`                                        | Raft term of the etcd cluster. Emitted periodically.                                                                                                                                                                             | number            |
| `ETCDReceivedBandwidthRate`                           | Number of bytes per second received by the follower etcd node. Emitted periodically.                                                                                                                                             | bytes             |
| `ETCDReceivedRequestRate`                             | Number of requests per second received by the follower etcd node. Emitted periodically.                                                                                                                                          | rate              |
| `ETCDSentBandwidthRate`                               | Number of bytes per second sent by the leader etcd node. Emitted periodically.                                                                                                                                                   | bytes             |
| `ETCDSentRequestRate`                                 | Number of requests per second sent by the leader etcd node. Emitted periodically.                                                                                                                                                | rate              |
| `ETCDWatchers`                                        | Number of watches set against the etcd cluster. Emitted periodically.                                                                                                                                                            | number            |
| `EncryptionDuration`                                  | Time the BBS took to ensure all BBS records are encrypted with the current active encryption key. Emitted each time a BBS becomes the active master.                                                                             | ns                |
| `LRPsClaimed`                                         | Total number of LRP instances that have been claimed by some cell. Emitted periodically.                                                                                                                                         | number            |
| `LRPsDesired`                                         | Total number of LRP instances desired across all LRPs. Emitted periodically.                                                                                                                                                     | number            |
| `LRPsExtra`                                           | Total number of LRP instances that are no longer desired but still have a BBS record. Emitted periodically.                                                                                                                      | number            |
| `LRPsMissing`                                         | Total number of LRP instances that are desired but have no record in the BBS.  Emitted periodically.                                                                                                                             | number            |
| `LRPsRunning`                                         | Total number of LRP instances that are running on cells. Emitted periodically.                                                                                                                                                   | number            |
| `LRPsUnclaimed`                                       | Total number of LRP instances that have not yet been claimed by a cell. Emitted periodically.                                                                                                                                    | number            |
| `LockHeld.` `v1-locks-bbs_lock`                          | Whether a BBS holds the BBS lock: 1 means the lock is held, and 0 means the lock was lost. Emitted periodically by the active BBS server.                                                                                        | 0 or 1 (boolean)  |
| `LockHeldDuration.` `v1-locks-bbs_lock`                  | Time the active BBS has held the BBS lock. Emitted periodically by the active BBS server.                                                                                                                                        | ns                |
| `MetricsReportingDuration`                            | Time it took to report periodic metrics.                                                                                                                                                                                         | ns                |
| `MigrationDuration`                                   | Time the BBS took to run migrations against its persistence store. Emitted each time a BBS becomes the active master.                                                                                                            | ns                |
| `RequestCount`                                        | Cumulative number of requests the BBS has handled through its API. Emitted periodically.                                                                                                                                         | number            |
| `RequestLatency`                                      | Time the BBS took to handle requests to its API endpoints. Emitted when the BBS API handles requests.                                                                                                                            | ns                |
| `TasksCompleted`                                      | Total number of Tasks that have completed. Emitted periodically.                                                                                                                                                                 | number            |
| `TasksPending`                                        | Total number of Tasks that have not yet been placed on a cell. Emitted periodically.                                                                                                                                             | number            |
| `TasksResolving`                                      | Total number of Tasks locked for deletion. Emitted periodically.                                                                                                                                                                 | number            |
| `TasksRunning`                                        | Total number of Tasks running on cells. Emitted periodically.                                                                                                                                                                    | number            |

## Converger

| Metric                                      | Description                                                                                                                                                | Unit             |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ----             |
| `LockHeld.` `v1-locks-converge_lock`           | Whether a converger holds the convergence lock: 1 means the lock is held, and 0 means the lock was lost. Emitted periodically by the active converger.     | 0 or 1 (boolean) |
| `LockHeldDuration.` `v1-locks-converge_lock`   | Time the active converger has held the convergence lock. Emitted periodically by the active converger.                                                     | ns               |

## Rep

| Metric                                               | Description                                                                                                                                                | Unit             |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ----             |
| `CapacityRemainingContainers`                        | Remaining number of containers this cell can host. Emitted periodically.                                                                                   | number           |
| `CapacityRemainingDisk`                              | Remaining amount of disk available for this cell to allocate to containers.  Emitted periodically.                                                         | bytes            |
| `CapacityRemainingMemory`                            | Remaining amount of memory available for this cell to allocate to containers.  Emitted periodically.                                                       | bytes            |
| `CapacityTotalContainers`                            | Total number of containers this cell can host. Emitted periodically.                                                                                       | number           |
| `CapacityTotalDisk`                                  | Total amount of disk available for this cell to allocate to containers. Emitted periodically.                                                              | bytes            |
| `CapacityTotalMemory`                                | Total amount of memory available for this cell to allocate to containers.  Emitted periodically.                                                           | bytes            |
| `ContainerCount`                                     | Number of containers hosted on the cell. Emitted periodically.                                                                                             | number           |
| `GardenContainerCreationDuration`                    | Time the rep's Garden backend took to create a container. Emitted after every successful container creation. (Deprecated)                                  | ns               |
| `GardenContainerCreationSucceededDuration`           | Time the rep's Garden backend took to create a container. Emitted after every successful container creation.                                               | ns               |
| `GardenContainerCreationFailedDuration`              | Time the rep's Garden backend took to create a container. Emitted after every failed container creation.                                                   | ns               |
| `GardenContainerDestructionSucceededDuration`        | Time the rep's Garden backend took to destroy a container. Emitted after every successful container destruction.                                           | ns               |
| `GardenContainerDestructionFailedDuration`           | Time the rep's Garden backend took to destroy a container. Emitted after every failed container destruction.                                               | ns               |
| `RepBulkSyncDuration`                                | Time the cell rep took to synchronize the ActualLRPs it has claimed with its actual garden containers. Emitted periodically by each rep.                   | ns               |
| `StalledGardenDuration`                              | Time the rep is waiting on its garden backend to become healthy during startup.  Emitted only if garden not responsive when the rep starts up.             | ns               |
| `StrandedEvacuatingActualLRPs`                       | Evacuating ActualLPRs that timed out during the evacuation process. Emitted when evacuation doesn't complete successful.                                   | number           |
| `UnhealthyCell`                                      | Whether the cell has failed to pass its healthcheck against the garden backend.  0 signifies healthy, and 1 signifies unhealthy. Emitted periodically.     | 0 or 1 (boolean) |
| `VolmanMountDuration`                                | Time volman took to mount a volume. Emitted by each rep when volumes are mounted.                                                                          | ns               |
| `VolmanMountErrors`                                  | Count of failed volume mounts. Emitted periodically by each rep.                                                                                           | number           |
| `VolmanUnmountDuration`                              | Time volman took to unmount a volume. Emitted by each rep when volumes are mounted.                                                                        | ns               |
| `VolmanUnmountErrors`                                | Count of failed volume unmounts. Emitted periodically by each rep.                                                                                         | number           |

## Route Emitter

| Metric                                         | Description                                                                                                                                                      | Unit             |
| -------------------------------------------    | ----------------------------------------------------------------------------------------------------------------------------------------------------------       | ----             |
| `AddressCollisions`                            | Number of detected conflicting routes. A conflicting route is a set of two distinct instances with the same IP address on the routing table.                     | number           |
| `LockHeld.` `v1-locks-route_emitter_lock`         | Whether a route-emitter holds the route-emitter lock: 1 means the lock is held, and 0 means the lock was lost. Emitted periodically by the active route-emitter. | 0 or 1 (boolean) |
| `LockHeldDuration.` `v1-locks-route_emitter_lock` | Time the active route-emitter has held the route-emitter lock. Emitted periodically by the active route-emitter.                                                 | ns               |
| `MessagesEmitted`                              | Cumulative number of messages the route-emitter sends over NATS to the gorouter.                                                                                 | number           |
| `RouteEmitterSyncDuration`                     | Time the active route-emitter took to perform its synchronization pass. Emitted periodically.                                                                    | ns               |
| `RoutesRegistered`                             | Cumulative number of route registrations emitted from the route-emitter as it reacts to changes to LRPs.                                                         | number           |
| `RoutesSynced`                                 | Cumulative number of route registrations emitted from the route-emitter during its periodic route-table synchronization.                                         | number           |
| `RoutesTotal`                                  | Number of routes in the route-emitter's routing table. Emitted periodically.                                                                                     | number           |
| `RoutesUnregistered`                           | Cumulative number of route unregistrations emitted from the route-emitter as it reacts to changes to LRPs.                                                       | number           |

## SSH Proxy

| Metric                                      | Description                                                                                                                                                | Unit   |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ----   |
| `ssh-connections`                           | Total number of SSH connections an SSH proxy has established. Emitted periodically by each SSH proxy.                                                      | number |

## General Golang metrics

These metrics are automatically emitted by dropsonde on all the Diego components.

| Metric                                      | Description                                                                                                                                                | Unit   |
| -------------------------------------------    | ----------------------------------------------------------------------------------------------------------------------------------------------------------       | ----             |
| `memoryStats.lastGCPauseTimeNS`             | Amount of time the Golang process paused for garbage collection.                                                                                           | ns     |
| `memoryStats.numBytesAllocatedHeap`         | Number of bytes the Golang process has allocated on the heap.                                                                                              | bytes  |
| `memoryStats.numBytesAllocatedStack`        | Number of bytes the Golang process has allocated on the stack.                                                                                             | bytes  |
| `memoryStats.numBytesAllocated`             | Total number of bytes allocated by the Golang process.                                                                                                     | bytes  |
| `memoryStats.numFrees`                      | Number of memory deallocations the Golang process has performed.                                                                                           | number |
| `memoryStats.numMallocs`                    | Number of memory allocations the Golang process has performed.                                                                                             | number |
| `numCPUS`                                   | Number of CPU cores available for the Golang process to use.                                                                                               | ns     |
| `numGoRoutines`                             | Number of goroutines the Golang process is running.                                                                                                        | number |
