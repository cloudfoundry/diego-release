# BBS Benchmarks

Results from running the
Diego [BBS benchmark tests](https://github.com/cloudfoundry/benchmarkbbs) on
the Diego team's time-rotor environment can be found below:

- Benchmark runs against CF-MySQL:
  * Raw results in the [S3 bucket](https://time-rotor-gcp-diego-benchmarks.s3.amazonaws.com/)
  * Metrics on the [Datadog dashboard](https://p.datadoghq.com/sb/ed32fa2e4-cdfe40bdd2)

- Benchmark runs against Postgresql:
  * Raw results in the [S3 bucket](https://time-rotor-gcp-diego-benchmarks-postgres.s3.amazonaws.com/)
  * Metrics on the [Datadog dashboard](https://p.datadoghq.com/sb/ed32fa2e4-f8c3ec44de)

The Datadog dashboard displays metrics for the results for all BBS benchmark runs on time-rotor in
the last 7 days.

Descriptions of the metrics from the benchmark runs are available in the
[BBS Benchmark documentation](https://github.com/cloudfoundry/benchmarkbbs#collected-metrics).
