# Migrating from Consul to SQL Locks (Experimental)

In order to migrate from the Consul backed lock to the SQL backed lock, we need to do a two phased deploy.
In the first deploy we enable SQL lock while keeping the Consul lock intact.
The second deploy will remove the consul backed lock, and exclusively use the SQL backed lock provided by locket.

## Deploy with both Consul and SQL

As a first step to enable the SQL backed lock, run the manifest generation script with the flag `-Q` as mentioned [here](manifest-generation.md#experimental--q-opt-into-using-sql-locket-service).
It is also **required** to create a certificate and key to be used by the SQL Lock. For more information on how to generate those certs, follow the instructions [here](tls-configuration.md). Also note that SQL locks require mutual TLS to be turned for all communication to the BBS. Again, refer to [this document](tls-configuration.md) for more information on how to turn on mutual TLS on the BBS.
After deploying the generated manifest, the locket server will be deployed co-located on the brain and database VM.
Also, the BBS and Auctioneer will obtain both the Consul lock through its local consul agent and the SQL lock through the co-located locket server.

**Note**: The components will always grab the consul lock prior to the SQL lock in order to prevent deadlock.

## Disable Consul Lock

In the final step, to disable Consul backed lock, set the following properties
to `true` in your property overrides stub:

* `property_overrides.bbs.skip_consul_lock`
* `property_overrides.auctioneer.skip_consul_lock`
* `property_overrides.tps.watcher.skip_consul_lock`

Afterwards, regenerate the deployment manifest, and redeploy the Diego deployment.
After the deployment of the regenerated manifest, the BBS and Auctioneer will now
only obtain the SQL lock through the co-located locket server.

