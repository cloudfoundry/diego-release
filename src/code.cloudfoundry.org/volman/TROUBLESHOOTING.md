# Troubleshooting Problems with Cloud Foundry Volume Services

## When the application does not start

- If you have pushed an app with `--no-start` and then bound the app to a service, the cf cli will tell you to `cf restage` to start the app with the new binding.  This is incorrect.  You must use `cf start` on an app that is not running.  It the app is already running, then `cf restage` is OK.
- If your application still won't start, try unbinding it from the volume service and see if it starts when it is *not* bound.  (Most of our test applications like [pora](https://github.com/cloudfoundry-incubator/persi-acceptance-tests/tree/master/assets/pora) and [kitty](https://github.com/EMC-Dojo/Kitty) will start up even when no volume is available.)
  If the application starts up, then that indicates the problem is volume services related.  If you still see the same error regardless, then that indicates that the problem is elsewhere, and you should go through the [Troubleshooting Application Deployment and Health](https://docs.cloudfoundry.org/devguide/deploy-apps/troubleshoot-app-health.html) steps.
- Simple errors may turn up in the application logs.  Try `cf logs <app name>`.  Even if there are no errors in evidence, make a note of the application guid in this log--it could be useful later.
- If you're specifying a `mount` in your bind configuration that overlaps with the application directory, you may encounter errors or odd behaviors at application startup because the mounts are established before the application droplet is unpacked into the container.  As a result, diego will attempt to unpack the app droplet into your mounted directory, and will fail if it cannot.  If you require the mounted directory to be available within your application directory tree, the preferred approach is to mount it elsewhere, and then create a symlink to it in your application's start script. 
- More detailed logging is available by restarting your app with `CF_TRACE`.  To do this, type  
   ```bash
   CF_TRACE=true cf restart <app name>
   ```
- If you see mount errors in the cf application logs when using the NFS volume service, it is possible that your NFS share is not opened to the Diego cells, or that the network access between the cell and the NFS server is not open.  To test this, you will need to SSH onto the cell.  See the steps below about failing broker/driver deployment for some information about how to bosh ssh into the cell.  Once you are ssh'd into the cell, type the following command to test NFS access:  
   ```bash
   mkdir foo
   sudo mount -t nfs -o vers=3 <nfs server>:<nfs share>
   sudo umount foo
   sudo mount -t nfs -o vers=4 <nfs server>:<nfs share>
   sudo umount foo
   ```  
   If the network is open, one or both of these commands should successfully mount. If neither works, check to make sure that your share is opened to the Diego cell IPs.  If only one of the mount commands succeeds, that suggests that the share is only open to NFS3 or NFS4 connections, and you will need to specify that version in your service configuration.
   You can employ a similar set of steps to dignose failures with SMB mounts, which may also need to be opened to Diego cell IPs.
- If you don't find any mount error, and your mount takes longer than 10s. It is possible that volman has canceled the connection to the volume driver which can't mount your NFS share in 10s. Check the network latency between your NFS share and Diego cells and DNS resolve speed.
- If you get this far, then you will need to consult the BOSH logs while restaging your application to see if you can find an error there (assuming that you have bosh access).  See the steps below about failing broker/driver deployment for some information about how to bosh ssh into the cell.  Once you are ssh'd into the cell, check the driver stderr/stdout logs.   It is also useful to look at the `rep` logs as some errors in the volume services infrastructure will end up there.
- If you don't see any errors on the Diego cell, it is likely that your error occurred in the cloud controller, before the could be placed on a cell.  To find the cloud controller logs related to your application, you can `bosh ssh` into the `api` vm in your cloudfoundry deployment.  `grep` for your application guid in the `cloud_controller_ng` logs.  Sometimes it is helpful to pipe the results of that `grep` to also grep for `error`:  
   ```bash
   grep <app guid> cloud_controller_ng.log | grep error
   ```

## When the application starts, but data is missing

If your application starts up, but it cannot find the data you expected in your share, it is possible that there is an issue with volume services--the volume will be mounted onto the diego cell, and then bind-mounted from the diego cell into the application container by garden.  Failures in either of those mounts that go undetected by the infrastructure could theoretically leave an empty directory in place of the volume mount, which could result in the appearance of an empty mount.  
However, it's a good idea to take a look on your application container to make sure that your volume mount is really placed where you expected it:
- `cf ssh <app name>` to enter the application container
- `echo $VCAP_SERVICES` to dump out the environment passed into the container by cloudfoundry.  In that data block you should see an entry called either `container_path` or `container_dir` (depending on your cloudfoundry version).  That will contain the path where your volume is mounted.
- `cd` to the path above, and validate that it contains the data you expected and/or that you can create files in that location.
- to double check that volume services are really working, you can bind a second app to the same service and `cf ssh` into that application.  If volume services are operational, data written in one application container will be in the share when you ssh into the other.

If your application requires data to be mounted in a specific location, you can normally alter the mount path when you bind your application to the volume by using the `-c` flag as follows;  
   ```bash
   cf bind-service <app name> <service name> -c '{"mount":"/path/in/container"}'
   ```
This mount configuration is supported by all of the volume service brokers in the `cloudfoundry-incubator` and `cloudfoundry` github orgs.

## When BOSH deployment fails

### Broker deployment (for bosh deployed brokers)

When broker deployment fails, assuming that Bosh has successfully parsed the manifest and created a vm for your broker, you will normally find any errors that occurred during startup by looking in the bosh logs.
Although you can gather the logs from your bosh vm using the `bosh logs` command, that command creates a big zip file with all the logs in it that muust be unpacked, so it is usually easier and faster to `bosh ssh` onto the vm and look at the logs in a shell.
Instructions for bosh ssh are [here](https://bosh.io/docs/sysadmin-commands.html#ssh).

Once you are ssh'd into the vm, switch to root with `sudo su` and then type `monit summary` to make sure that your broker job is really not running.
Assuming that the broker is not showing as running, you should see some type of error in one of three places:
- `/var/vcap/sys/log/monit/` contains monit script output for the various bosh logs.  Errors that occur in outer monit scripts will appear here.
- `/var/vcap/sys/log/packages/<broker name>` contains package installation logs for your broker source.  Some packaging errors end up here  
- `/var/vcap/sys/log/jobs/<broker name>` contains logs for your actual broker process.  Any errors from the running executable or pre-start script will appear in this directory.

### Driver deployment

Diagnosing failures in driver deployment is quite similar to bosh deployed broker diagnosis as described above.  The principal difference is that the driver is deployed alongside diego, so you must ssh into the diego-cell VM to find the driver job.  
In a multi-cell deployment, sometimes it is necessary to try different cell vms to find the failed one, but most of the time if configuration is not right, all cells will fail in the same way.
