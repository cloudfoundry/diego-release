#!/usr/bin/env bash

function initialize_mysql {
  cat << EOF > /etc/my.cnf
[mysqld]
sql_mode=NO_ENGINE_SUBSTITUTION,STRICT_TRANS_TABLES
EOF
  datadir=/mysql-datadir
  escaped_datadir=${datadir/\//\\\/}
  mkdir $datadir
  mount -t tmpfs -o size=2g tmpfs $datadir
  rsync -av --progress /var/lib/mysql/ $datadir
  sed -i "s/datadir.*/datadir=${escaped_datadir}/g" /etc/mysql/mysql.conf.d/mysqld.cnf
  service mysql start
}
