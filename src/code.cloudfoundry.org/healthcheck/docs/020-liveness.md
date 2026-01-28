---
title: Liveness Healthchecks
expires_at : never
tags: [diego-release, healthcheck]
---

### Liveness Healthcheck

```
# HTTP Liveness Healthcheck
./healthcheck -uri=URI \
     -liveness-interval=INTERVAL \
     [-port=PORT]
     [-timeout=TIMEOUT]

# TCP Liveness Healthcheck
./healthcheck \
     -liveness-interval=INTERVAL \
     [-port=PORT]
     [-timeout=TIMEOUT]
```

| Flag | Default | Description |
|---|---|---|
| uri | no default | URI to healthcheck. Required for HTTP healthchecks. |
| port | 8080 | Port to healthcheck.  |
| timeout | 1s  | Dial timeout when connecting to app. |
| liveness-interval | 0s | If set, starts the healthcheck in liveness mode, i.e. the app is alive and hasn't crashed, do not exit until the healthcheck fails. runs checks every liveness-interval. Required for liveness healthchecks. |

The Liveness healthcheck should be used once the app has passed the startup
healthcheck. This healthcheck will return non-zero when the healthcheck gets a
failure response. This indicates that the app was running, but something has
gone wrong. As long as the healthcheck keeps getting a healthy response from
the app, then it will not stop running.
