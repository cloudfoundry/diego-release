---
title: Diego Logging Format 
expires_at : never
tags: [diego-release]
---

# Diego Logging Format

Diego components log using a JSON format provided by the [lager](https://github.com/cloudfoundry/lager) library.

It's possible to change the format of the timestamps by setting the `logging.format.timestamp` property to `rfc3339` for the following jobs:

- `auctioneer`
- `bbs`
- `file_server`
- `locket`
- `rep`
- `rep_windows`
- `route_emitter`
- `route_emitter_windows`
- `ssh_proxy`

Enabling these human readable timestamps also changes the format of the `log_level` field.

## Example

`logging.format.timestamp: "unix-epoch"`:

`{"timestamp":"1522171784.100530624","source":"rep","message":"rep.started","log_level":1,"data":{"cell-id":"7f84c2ea-ce4a-43cc-920d-db8d8d66f58e"}}`

`logging.format.timestamp: "rfc3339"`:

`{"timestamp":"2018-03-26T23:57:03.5858943Z","level":"info","source":"rep","message":"rep.started","data":{"cell-id":"347c58b7-0c4a-419f-8126-21e0882d6b15"}}`



