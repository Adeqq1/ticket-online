#!/bin/sh
set -eu
mysql --protocol=socket -uroot -p"$MYSQL_ROOT_PASSWORD" <<SQL
CREATE USER IF NOT EXISTS 'ticket_backup'@'%' IDENTIFIED BY '${STAGING_DB_BACKUP_PASSWORD}';
GRANT SELECT, SHOW VIEW ON \`${MYSQL_DATABASE}\`.* TO 'ticket_backup'@'%';
SQL
