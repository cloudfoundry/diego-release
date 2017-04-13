# Performance Tuning Recommendations

This document describes recommendations for performance tuning of the Diego Data Store.


### Table of Contents

1. [MySQL Performance Tuning](#mysql-performance-tuning)


### <a name="mysql-performance-tuning"></a> MySQL Performance Tuning

Operators can set the following values in the case of a high traffic deployment:

* Set the `innodb_flush_log_at_trx_commit` to `0` so that the log buffer is
  written to the log file approximately every second. For more details check
  the
  [MySQL manual](https://dev.mysql.com/doc/refman/5.7/en/innodb-parameters.html#sysvar_innodb_flush_log_at_trx_commit)

* If you are
  using [CF Mysql Release](https://github.com/cloudfoundry/cf-mysql-release),
  then set the `cf_mysql.mysql.innodb_flush_log_at_trx_commit` in the
  deployment mainfest to `0`.
