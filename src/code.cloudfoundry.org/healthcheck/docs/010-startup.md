---
title: Startup Healthchecks
expires_at : never
tags: [diego-release, healthcheck]
---

### Startup Healthcheck

```
# HTTP Startup Healthcheck
./healthcheck -uri=URI \
     -startup-interval=INTERVAL \
     [-port=PORT]
     [-timeout=TIMEOUT] \
     [-startup-timeout=STARTUP_TIMEOUT]

# TCP Startup Healthcheck
./healthcheck \
     -startup-interval=INTERVAL \
     [-port=PORT]
     [-timeout=TIMEOUT] \
     [-startup-timeout=STARTUP_TIMEOUT]
```

| Flag | Default | Description |
|---|---|---|
| uri | no default | URI to healthcheck. Required for HTTP healthchecks. |
| port | 8080 | Port to healthcheck.  |
| timeout | 1s  | Dial timeout when connecting to app. |
| startup-interval  | 0s | If set, starts the healthcheck in startup mode, i.e. do not exit until the healthcheck passes. Runs checks every startup-interval. Required for startup healthchecks. |
| startup-timeout  | 60s  | Only relevant if healthcheck is running in startup mode. When the timeout is set to a non-zero value, the healthcheck will return non-zero with any errors if this timeout is hit without the healthcheck passing. |

The startup healthcheck should be used when an app is starting up. It will
return zero when the healthcheck gets a successful response. It will return
non-zero when it does not get a successful response within the timeouts; this
means that the app did not start in the timeout provided.
