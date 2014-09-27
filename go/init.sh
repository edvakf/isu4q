#!/bin/sh
set -x
set -e
cd $(dirname $0)

myuser=root
mypass=isucon
mydb=isu4_qualifier
myhost=127.0.0.1
myport=3306
mysql -h ${myhost} -P ${myport} -u ${myuser} -e "DROP DATABASE IF EXISTS ${mydb}; CREATE DATABASE ${mydb}"
mysql -h ${myhost} -P ${myport} -u ${myuser} ${mydb} < /home/isucon/webapp/go/sql/schema.sql
mysql -h ${myhost} -P ${myport} -u ${myuser} ${mydb} < /home/isucon/webapp/go/sql/dummy_users.sql
mysql -h ${myhost} -P ${myport} -u ${myuser} ${mydb} < /home/isucon/webapp/go/sql/dummy_log.sql
mysql -h ${myhost} -P ${myport} -u ${myuser} ${mydb} < /home/isucon/webapp/go/sql/alter.sql

curl http://localhost/init
