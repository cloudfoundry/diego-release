# Troubleshooting error responses

## Components exiting due to Locket Lock Request Failures

Diego components that use Locket locks to maintain a single active node may
exit due to failed requests to the Locket server to refresh their lock.
Some such errors are cryptic and need some translating as they are often
directly returned from gRPC.

Example of a BBS exit due to lock loss:

```
{"timestamp":"2019-11-07T20:45:18.942850421Z","level":"error","source":"bbs","message":"bbs.exited-with-failure","data":{"error":"Exit trace for group:\nlock exited with error: rpc error: code = DeadlineExceeded desc = context deadline exceeded\ndb-stat-metron-notifier exited with nil\ntask-stat-metron-notifier exited with nil\nlrp-stat-metron-notifier exited with nil\nconverger exited with nil\nperiodic-metrics exited with nil\nbbs-election-metrics exited with nil\nhub-maintainer exited with nil\nencryptor exited with nil\nmigration-manager exited with nil\nserver exited with nil\nworkpool exited with nil\nset-lock-held-metrics exited with nil\nlock-held-metrics exited with nil\nperiodic-filedescript-(truncated)"}}
```

Components like the BBS are constructed with a series of
[ifrit](https://github.com/tedsuo/ifrit) processes in a group. When one exits,
the whole group is torn down and the result of each of these processes is
returned and is what you see in a log line as above. In this example of the 
BBS locket client process exiting with failure, we can see the error message is
joined with the results of the other ifrit processes in the BBS (in the
`data.error` field of the log line). Below is a table that can be used to
classify the errors that may result from the locket client process exiting:

| Error Message Content | Interpretation |
| --------------------- | -------------- |
| `DeadlineExceeded`/`context deadline exceeded` | The request to the Locket server did not complete before the client timeout. Possible issues include: DNS resolution, slow network, Locket database instability/load, insufficient Locket server CPU resources | 
| `AlreadyExists`/`lock-collision` | The component making the request to refresh its lock received an error that the lock is already owned. This should not happen and could be caused by database instability/inconsistency or database load |

**Note** For more details on the errors you may see in component logs, see
[this documentation](https://godoc.org/google.golang.org/grpc/codes)
