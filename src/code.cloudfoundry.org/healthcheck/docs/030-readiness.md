---
title: Readiness Healthchecks
expires_at : never
tags: [diego-release, healthcheck]
---

There are two types of readiness healthchecks.

### Until Ready Readiness Healthcheck

```
# HTTP Until Ready Readiness Healthcheck
./healthcheck -uri=URI \
     -until-ready-interval=INTERVAL \
     [-port=PORT]
     [-timeout=TIMEOUT]

# TCP Until Ready Readiness Healthcheck
./healthcheck \
     -until-ready-interval=INTERVAL \
     [-port=PORT]
     [-timeout=TIMEOUT]
```

| Flag | Default | Description |
|---|---|---|
| uri | no default | URI to healthcheck. Required for HTTP healthchecks. |
| port | 8080 | Port to healthcheck.  |
| timeout | 1s  | Dial timeout when connecting to app. |
| until-ready-interval | 0s | If set, starts the healthcheck in until-ready mode, i.e. do not exit until the healthcheck passes and the app is ready to serve traffic. Runs checks every until-ready-interval. Required for until ready readiness healthchecks. |

The until ready readiness healthcheck will return zero when the healthcheck
gets a successful response. This indicates that the app is running and ready to
be routed to. As long as the healthcheck keeps getting a failure response from
the app, then it will not stop running.


### Until Failure Readiness Healthcheck

```
# HTTP Until Failure Readiness Healthcheck
./healthcheck -uri=URI \
     -readiness-interval=INTERVAL \
     [-port=PORT]
     [-timeout=TIMEOUT]

# TCP Until Failure Readiness Healthcheck
./healthcheck \
     -readiness-interval=INTERVAL \
     [-port=PORT]
     [-timeout=TIMEOUT]
```

| Flag | Default | Description |
|---|---|---|
| uri | no default | URI to healthcheck. Required for HTTP healthchecks. |
| port | 8080 | Port to healthcheck.  |
| timeout | 1s  | Dial timeout when connecting to app. |
| readiness-interval | 0s | If set, starts the healthcheck in readiness mode, i.e. the app is ready to serve traffic, i.e. do not exit until the healthcheck fails because the target isn't serving traffic or another process doesn't exist. Runs checks every readiness-interval. Required for until failure readiness healthchecks. |

The until ready failure healthcheck will return non-zero when the healthcheck
gets a failure response. This indicates that the app is no longer ready to be
routed to. As long as the healthcheck keeps getting a success response from the
app, then it will not stop running.
