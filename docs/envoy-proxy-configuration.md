# Envoy Proxy Configuration

This document describes how to enable the per-container [Envoy proxy](https://github.com/envoyproxy/envoy) on Linux Diego cells, how its ports are configured to proxy incoming TCP traffic, and how operators can configure an additional memory allocation for each container to compensate for its memory usage. 

## Table of Contents

1. [Enabling Per-Container Envoy Proxy](#enabling-per-container-envoy-proxy)
1. [Envoy Proxy Configuration for Route Integrity](#envoy-proxy-configuration-for-route-integrity)
1. [Additional Per-Instance Memory Allocation](#additional-per-instance-memory-allocation)
	1. [Choosing a value for the additional memory allocation](#choosing-value-for-additional-memory-allocation)
1. [Enabling Mutual TLS Configuration](#enabling-mutual-tls-configuration)
1. [Disabling Unproxied Port Mappnigs](#disabling-unproxied-port-mappings)


## <a name="enabling-per-container-envoy-proxy"/> Enabling Per-Container Envoy Proxy

A deployment operator enables the Linux cell reps to run an Envoy proxy process in each container by setting the `containers.proxy.enabled` property on the `rep` job to `true`.

[Instance Identity Credentials](https://docs.cloudfoundry.org/adminguide/instance-identity.html) must also be enabled on the Diego cell rep so that it can configure the Envoy proxy process with the required TLS configuration.


## <a name="envoy-proxy-configuration-for-route-integrity"/> Envoy Proxy Configuration for Route Integrity

The Diego cell rep configures Envoy primarily to support improved resilience and security of the routing tiers in Cloud Foundry via [TLS to the app container](https://docs.cloudfoundry.org/concepts/http-routing.html#with-tls). For each port value specified to route into the container, the cell rep selects a different port on which Envoy will listen for TCP connections and then proxy traffic to the original container port. The cell rep selects these ports from 61001 and above, to minimize conflicts with both user-bound service ports (typically in the 1-32767 range) and ephemeral ports (typically 32768-61000).

As an example, suppose that the DesiredLRP for a CF app specifies the following list of ports to be open for incoming traffic:

- port 8080, for HTTP traffic to the main web UI for the app;
- port 9999, for TCP traffic to a monitoring port;
- port 2222, for CF SSH access.

The cell rep then may select the following ports on which to configure Envoy to listen:

- port 61001, proxying traffic to port 8080;
- port 61002, proxying traffic to port 9999;
- port 61003, proxying traffic to port 2222.

Envoy uses the instance-identity credentials on each one of these ports to terminate TLS before proxying traffic to the appropriate destination port in the container. The listeners also listen on all available interfaces by binding to the `0.0.0.0` address; in practice, this means they listen on localhost (`127.0.0.1`) and on the IP address for the container's `eth0` interface.

At present, these TLS-proxied ports may differ from instance to instance of even the same app. The route-registration system already registers these ports dynamically with the Gorouters and other Cloud Foundry routing tiers, so consistency across instances of the same app has not been important for that use case. When an instance is running on a Diego cell with the Envoy proxy enabled, it will find these additional port mappings in the `internal_tls_proxy` and `external_tls_proxy` fields in the entries in the `CF_INSTANCE_PORTS` environment variable. Developers can also inspect this value manually from a `cf ssh` session:

```
% cf ssh APP_NAME
vcap@b66d0aa2-cf1f-4b4e-6029-b6c5:~$ echo $CF_INSTANCE_PORTS | jq .
[
  {
    "internal_tls_proxy": 61001,
    "external_tls_proxy": 61011,
    "internal": 8080,
    "external": 61008
  },
  {
    "internal_tls_proxy": 61002,
    "external_tls_proxy": 61012,
    "internal": 9999,
    "external": 61009
  },
  {
    "internal_tls_proxy": 61003,
    "external_tls_proxy": 61013,
    "internal": 2222,
    "external": 61010
  }
]
```

Additionally, the cell rep configures the Envoy proxy to bind its administrative API server to another port in the 61001+ range, but only on the localhost interface.


## <a name="additional-per-instance-memory-allocation"/> Additional Per-Instance Memory Allocation

Integrating Envoy into Cloud Foundry for route integrity and other use cases does not come for free. Each Envoy proxy process currently runs in the same container as the application process and so itself uses memory inside the application instance's memory quota. Consequently, operators will likely be interested in compensating for this overhead to reduce the incidence of out-of-memory errors inside application containers, and can do so by setting the `containers.proxy.additional_memory_allocation_mb` property on the `rep` job.

This property instructs the cell rep to increase the memory quota of all containers by the given amount. For example, if `containers.proxy.additional_memory_allocation_mb` is set to `16` and a container is scheduled on the cell with a memory limit of 256 MB, the actual container effective memory limit will be 256 + 16 = 272 MB. Please note that the cell rep will not overcommit its memory as a result of setting the bosh property, and operators opting into this mode should make sure that they have enough additional memory.

Container memory usage metrics sent through the Loggregator system and exposed on the cell rep's container metrics API endpoint are rescaled linearly to fit within the externally requested quota, to reduce confusion about the source of out-of-memory errors.


### <a name="choosing-value-for-additional-memory-allocation"/> Choosing a value for the additional memory allocation

The Diego team has in [story #155945585](https://www.pivotaltracker.com/story/show/155945585) done some investigation of how the Envoy proxy uses memory in practice, and as of Envoy version `0e1d66377d9bf8b8304b65df56a4c88fc01e87e8` has determined that Envoy initially uses between 5 and 10 MB of memory, and then uses approximately 30KB of memory per concurrent connection. The memory usage also remains at that level even if the number of concurrent connections decreases. Consequently, if operators have an estimate of `N` for the maximum number of concurrent connections from the gorouters to a single app instance, this assessment suggests that the `containers.proxy.additional_memory_allocation_mb` property should be set to the value `10 + 0.03 * N` (rounded to the nearest integer). This additional allocation may of course need to be adjusted according to the specifics of the applications running in each environment.

### <a name="enabling-mutual-tls-configuration"/> Enabling Mutual TLS Configuration

A deployment operator can enable mutual TLS configuration between the Envoy proxy which runs in the application container and the Gorouter by performing the following steps:

1. Create the set of CA certificate, client certificate, and a private key.
1. In the `gorouter` job for the `routing-release`, set the values of `router.backends.cert_chain` and `router.backends.private_key` properties to the certificate and private key generated in the step above.
1. In the `rep` job for `diego-release`, set the `containers.proxy.require_and_verify_client_certificates` property to `true`.
1. In the `rep` job, also set the value of `containers.proxy.trusted_ca_certificates` to the CA certificate created in the first step.
1. Optionally, you can configure the Envoy proxy to validate the subject alternative name on the certificate provided by the gorouter. To do so, the certificate template needs to contain the subject alternative name, and that same name can be set in `containers.proxy.verify_subject_alt_name` in the `rep` job.

### <a name="disabling-unproxied-port-mappings"/> Disabling Unproxied Port Mappnigs

A deployment operator can also disable the legacy port mappings that bypass the Envoy proxy by setting the `containers.proxy.enable_unproxied_port_mappings` property on the `rep` job to `false`. Setting this value requires the Envoy proxies to be enabled.

Note that this configuration is compatible with only the Cloud Foundry routing tiers that support TLS connections to backend instances, which to date includes the HTTP gorouters but does **not** include either the TCP routers or the Diego SSH proxies.

Together with enabling mutual TLS configuration, this configuration can be used to ensure that only certain authenticated clients can send TCP traffic to the application servers inside Diego LRP and CF app containers.
