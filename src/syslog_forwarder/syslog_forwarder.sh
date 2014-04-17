#!/usr/bin/env sh

if [ $# -ne 1 ]
then
    echo "Usage: syslog_forwarder.sh [rsyslog config file]"
    exit 1
fi

CONFIG_FILE=$1

# Place to spool logs if the upstream server is down
mkdir -p /var/vcap/sys/rsyslog/buffered
chown -R syslog:adm /var/vcap/sys/rsyslog/buffered

cp $CONFIG_FILE /etc/rsyslog.d/00-syslog_forwarder.conf

/usr/sbin/service rsyslog restart
