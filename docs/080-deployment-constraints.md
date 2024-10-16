---
title: Deployment Constraints
expires_at : never
tags: [diego-release]
---

# Deployment Constraints

## <a name="required-dependencies"></a>Required Dependencies

Before deploying the Diego cluster, ensure that the sql database it will connect to is already deployed and provisioned.

## <a name="diego-manifest-jobs"></a>Diego Manifest Jobs

In your manifest, ensure that the following constraints on job update order and rate are met:

1. BBS servers should update before BBS clients. This can be achieved by placing `database_zN` instances at the beginning of the jobs list in your manifest. For example:

	```
	jobs:
	- instances: 1
	  name: database_z1
	```

1. `database_zN` nodes update one at a time. This can be achieved by setting `max_in_flight` to `1` and `serial` to `true` for `database_zN` jobs.

	```
	- instances: 1
	  name: database_z1
	  ...
	  update:
	    max_in_flight: 1
	    serial: true
	```

1. `brain_zN` jobs update separately from cells. This can be achieved by setting `max_in_flight` to `1` and `serial` to `true` for `brain_zN` jobs.

	```
	- instances: 1
	  name: brain_z1
	  ...
	  update:
	    max_in_flight: 1
	    serial: true
	```

