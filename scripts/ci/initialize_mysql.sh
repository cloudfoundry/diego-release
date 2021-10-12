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

function bootDB {
  db=$1

  if [ "$db" = "random" ]; then
    cointoss=$RANDOM
    set +e
    let "cointoss %= 2"
    set -e
    if [ "$cointoss" == "0" ]; then
      db="postgres"
    else
      db="mysql"
    fi
  fi

  if [ "$db" = "postgres" ]; then
    launchDB="(docker-entrypoint.sh -c max_connections=300 &> /var/log/postgres-boot.log) &"
    testConnection="psql -h localhost -U $POSTGRES_USER -c '\conninfo' &>/dev/null"
  elif [[ "$db" == "mysql"* ]]; then
    chown -R mysql:mysql /var/run/mysqld
    launchDB="(MYSQL_USER='' MYSQL_ROOT_PASSWORD=$MYSQL_PASSWORD /entrypoint.sh mysqld &> /var/log/mysql-boot.log) &"
    testConnection="echo '\s;' | mysql -h127.0.0.1 -uroot --password=$MYSQL_PASSWORD &>/dev/null"
  else
    echo "skipping database"
    return 0
  fi

  echo -n "booting $db"
  eval "$launchDB"
  for _ in $(seq 1 60); do
    set +e
    eval "${testConnection}"
    exitcode=$?
    set -e
    if [ ${exitcode} -eq 0 ]; then
      echo "connection established to $db"
      return 0
    fi
    echo -n "."
    sleep 1
  done
  echo "unable to connect to $db"
  exit 1
}
