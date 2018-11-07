# Understanding Diego Logs for Pushing an Application

The goal of this tutorial is to provide a framework to help the reader debug issues encountered by the many components that make up Diego.

This tutorial will use a very common flow, pushing an application on the Cloud Foundry platform, to demonstrate the sequence in which Diego components work together to ultimately run the application within container(s).

First, important terminology...

## Terminology

### Workloads
Workloads are simply processes that are run in containers. There are two types of processes:

  - **LRP** (Long Running Process)
    A LRP is a process which is expected to run for an infinite amount of time, for example an application server.

  - **Task**
    A Task is a representation of a process which is guaranteed to be run at most once.

### Droplet
A Droplet is a bundle that contains an application that can run on Cloud Foundry.
The Cloud Controller generates Droplets by compiling application code in a staging Task.

### BBS (Bulletin Board System):
BBS is the external API server that accepts requests from Cloud Controller to run workloads in containers.

### Cell Rep:
A Cell Rep is an internal server that ultimately runs the work that is sent to the BBS in containers. To do this, it manages the orchestration of containers on the virtual machine it is installed on. Typically, there will be more than one Cell Rep because the number of Cell Reps will scale horizontally with the amount of work Diego is expected to do.

### Garden:
Garden is an internal server that exposes an API for doing basic operations with containers. The Cell Rep uses Garden to perform its more complex operations.

### Auctioneer
Auctioneer is an internal server that receives the work given to the BBS, and distributes this work in an intelligent way to Cell Reps.

### Desired LRP
A Desired LRP represents a workload from the Cloud Controller. Cloud controller can convey with a single Desired LRP that it wants to run multiple instances of this long running process.

### Actual LRP
An Actual LRP represents a single instance of a long running process that could be run in a container on the Cell Rep's virtual machine.

## Pushing an app on Cloud Foundry

Make sure these prerequisites are satisfied:
1. a Cloud Foundry is deployed
1. the Cloud Foundry CLI, `cf`, is installed
1. target the Cloud Foundry's API endpoint with the CLI
1. log in to Cloud Foundry with the CLI
1. target an organization and space with the CLI
1. the BOSH CLI for collecting logs for the app

This tutorial will push the [sample http app](https://github.com/cloudfoundry/sample-http-app):

```
cf push test-app -p <path_to_sample_http_app>
```

## Getting app logs

Ultimately, we want to inspect the logs from the BBS, Auctioneer, and Cell Rep(s). There are a lot of logs and many are not related to the app that was pushed. To filter these logs, we're going to need unique identifiers that can be obtained by:

```
cf logs test-app --recent
```

For a successful push, the output looks like this:
```
...
2018-11-05T13:26:36.65-0800 [API/0] OUT Created app with guid <**AppGUID**>
...
2018-11-05T13:26:44.67-0800 [STG/0] OUT Cell <**CellGUIDForStagingTask**> successfully created container for instance <**InstanceGUIDForStagingTask**>
...
2018-11-05T13:27:13.60-0800 [CELL/0] OUT Cell <**CellGUIDForApp**> successfully created container for instance <**InstanceGUIDForApp**>
...
```

- **AppGUID** is the unique identifier Cloud Foundry assigned to the app
- **CellID** is the unique identifier of the Cell Rep that will manage running the app in a container
- **InstanceGUID** is the unique identifier of the container running the app

There are two requests that the Cloud Controller makes to the BBS:
1. Cloud Controller will send a staging Task to the BBS to generate the
	 Droplet. The BBS will ask the Auctioneer to put the Task up for auction. The
	 Auctioneer will find a suitable Cell Rep (`CellGUIDForStagingTask`) and
	 request that the Cell Rep run the Task in a container
	 (`InstanceGUIDForStagingTask`).

2. Once a droplet is generated, Cloud Controller will ask the BBS to run the
	 desired LRP for the application. A similar process occurs between the BBS,
	 the Auctioneer, and the suitable Cell Rep (`CellGUIDForApp`) to run the LRP
	 in a container (`InstanceGUIDForApp`).

### Understanding BBS Logs

Now that we have the **AppGuid**, we can filter and extract the application related logs starting with the BBS:

```
bosh -d cf ssh diego-api -c "zgrep \"<AppGUID>\" /var/vcap/sys/log/bbs/bbs.stdout.log"
```

The log lines are in JSON format and can be parsed with a JSON prettifier tool like `jq`.

Looking at the `"message"` field in the returned JSON objects, we should be
able to identify the following sequence of actions taken by the BBS. To
simplify the explanation, we have omitted the logs corresponding to the Staging Task.

First, the BBS creates a DesiredLRP to represent an LRP desired by Cloud Controller.
At this point, a container has not yet been created to run the application.

```
"bbs.request.desire-lrp.starting"
"bbs.request.desire-lrp.complete"
```

In order to reconcile the difference between the set of DesiredLRPs and the
set of ActualLRPs, the BBS will create an ActualLRP in the `UNCLAIMED` state.
The BBS will also ask the Auctioneer to put the ActualLRP up for auction.

```
"bbs.request.desire-lrp.start-instance-range.starting"
"bbs.request.desire-lrp.start-instance-range.create-unclaimed-actual-lrp.starting"
"bbs.request.desire-lrp.start-instance-range.create-unclaimed-actual-lrp.complete"
"bbs.request.desire-lrp.start-instance-range.start-lrp-auction-request"
"bbs.request.desire-lrp.start-instance-range.finished-lrp-auction-request"
"bbs.request.desire-lrp.start-instance-range.complete"
```

Once the Auctioneer finds a suitable Cell Rep, it will assign the ActualLRP to
that Cell Rep. When the Cell Rep accepts the ActualLRP from the Auctioneer, it
notifies the BBS that it has claimed the responsibility of running that
ActualLRP. This will change the state of the LRP to the `CLAIMED` state.


```
"bbs.request.claim-actual-lrp.starting"
"bbs.request.claim-actual-lrp.complete"
```

Once the droplet has started running in a container, this Cell Rep will notify
the BBS that ActualLRP is started. This will change the state of the LRP to the
`RUNNING` state.

```
"bbs.request.start-actual-lrp.starting"
"bbs.request.start-actual-lrp.completed"
```

### Understanding Auctioneer Logs

Get the Auctioneer logs with:

```
bosh -d cf ssh scheduler -c "cat /var/vcap/sys/log/auctioneer/auctioneer.stdout.log" | jq .message
```

The Auctioneer gets the state of all the Cell Rep(s) and performs its auction
algorithm to find a suitable Cell Rep to assign the ActualLRP.

```
"auctioneer.request.serving"
"auctioneer.request.lrp-auction-handler.create.submitted"
"auctioneer.request.done"
"auctioneer.auction.fetching-cell-reps"
"auctioneer.auction.fetched-cell-reps"
"auctioneer.auction.fetching-zone-state"
"auctioneer.auction.fetched-cell-state"
"auctioneer.auction.fetched-cell-state"
"auctioneer.auction.zone-state"
"auctioneer.auction.zone-state"
"auctioneer.auction.fetched-zone-state"
"auctioneer.auction.fetching-auctions"
"auctioneer.auction.fetched-auctions"
"auctioneer.auction.scheduling"
"auctioneer.auction.scoring-lrp"
"auctioneer.auction.proxied-lrp"
"auctioneer.auction.scoring-lrp"
"auctioneer.auction.proxied-lrp"
"auctioneer.auction.lrp-added-to-cell"
"auctioneer.auction.scheduled"
```

### Understanding Cell Rep Logs

Using the **CellID** and **InstanceGUID** we obtained earlier, we can filter and extract the application related logs from the Cell Rep that was assigned the ActualLRP during auction:

```
bosh -d cf ssh diego-cell/<CellID>` -c "zgrep \"<InstanceGUID>\" /var/vcap/sys/log/rep/rep.stdout.log" | jq .message
```

First, Cell Rep creates a representation of a Garden container. Then it asks
Garden to create an empty container (the prefix
`rep.executing-container-operation.ordinary-lrp-processor.process-reserved-container.run-container`
has been truncated for easier reading):

```
".creating-container"
".containerstore-create.starting"
".containerstore-create.node-create.cached-dependency-rate-limiter"
".containerstore-create.node-create.downloader.acquire-rate-limiter.starting"
".containerstore-create.node-create.downloader.acquire-rate-limiter.completed"
".containerstore-create.node-create.downloader.file-cache.get-directory.starting"
".containerstore-create.node-create.downloader.file-cache.get-directory.finished"
".containerstore-create.node-create.downloader.download.starting"
".containerstore-create.node-create.downloader.download.download-barrier"
".containerstore-create.node-create.downloader.download.fetch-request"
".containerstore-create.node-create.downloader.download.completed"
".containerstore-create.node-create.downloader.directory-found-in-cache"
".containerstore-create.node-create.adding-container-proxy-bindmounts"
".containerstore-create.node-create.adding-healthcheck-bindmounts"
".containerstore-create.node-create.creating-container-in-garden"
".containerstore-create.node-create.created-container-in-garden"
".containerstore-create.complete"
".succeeded-creating-container-in-garden"
```

Next, the Cell Rep downloads dependencies required to run the droplet in the
container. Then it starts the application process in the container (the prefix
`rep.executing-container-operation.ordinary-lrp-processor.process-reserved-container.run-container`
has been truncated for easier reading):

```
".running-container-in-garden"
".containerstore-run.starting"
".containerstore-run.node-run.transform-check-definitions-starting"
".containerstore-run.node-run.transform-check-definitions-finished"
".containerstore-run.complete"
".succeeded-running-container-in-garden"
".containerstore-run.node-run.cred-manager-runner.starting"
".containerstore-run.node-run.cred-manager-runner.generating-credentials.starting"
".containerstore-run.node-run.cred-manager-runner.generating-credentials.complete"
".containerstore-run.node-run.cred-manager-runner.started"
".containerstore-run.node-run.setup.download-step.acquiring-limiter"
".containerstore-run.node-run.setup.download-step.acquired-limiter"
".containerstore-run.node-run.setup.download-step.fetch-starting"
".containerstore-run.node-run.setup.download-step.downloader.acquire-rate-limiter.starting"
".containerstore-run.node-run.setup.download-step.downloader.acquire-rate-limiter.completed"
".containerstore-run.node-run.setup.download-step.downloader.file-cache.get.starting"
".containerstore-run.node-run.setup.download-step.downloader.file-cache.get.finished"
".containerstore-run.node-run.setup.download-step.downloader.download.starting"
".containerstore-run.node-run.setup.download-step.downloader.download.download-barrier"
".containerstore-run.node-run.setup.download-step.downloader.download.fetch-request"
".containerstore-run.node-run.setup.download-step.downloader.download.completed"
".containerstore-run.node-run.setup.download-step.downloader.file-found-in-cache"
".containerstore-run.node-run.setup.download-step.fetch-complete"
".containerstore-run.node-run.setup.download-step.stream-in-starting"
".containerstore-run.node-run.setup.download-step.stream-in-complete"
".containerstore-run.node-run.action.run-step.running"
".containerstore-run.node-run.action.run-step.running"
".containerstore-run.node-run.envoy-readiness-check.run-step.running"
".containerstore-run.node-run.envoy-readiness-check.run-step.running"
".containerstore-run.node-run.readiness-check.run-step.running"
".containerstore-run.node-run.proxy.run-step.running"
".containerstore-run.node-run.envoy-readiness-check.run-step.process-exit"
".containerstore-run.node-run.envoy-readiness-check.run-step.process-exit"
".containerstore-run.node-run.readiness-check.run-step.process-exit"
".containerstore-run.node-run.health-check-step.transitioned-to-healthy"
".containerstore-run.node-run.liveness-check.run-step.running"
```

After the application is running, the Cell Rep notifies the BBS that the corresponding ActualLRP is running. This will change the state of the ActualLRP to the `RUNNING` state.

```
"rep.executing-container-operation.ordinary-lrp-processor.process-running-container.bbs-start-actual-lrp"
```

## Conclusion

Now that you have a basic framework for navigating the Diego components through
the happy path of pushing an application, we encourage you to dig around the
logs.
