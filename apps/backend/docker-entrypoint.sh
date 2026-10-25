#!/bin/sh
set -eu
mkdir -p /data/meta /data/backups
chown -R app:app /data
if [ -d /sqlite-sources ] && [ -w /sqlite-sources ]; then
  chown app:app /sqlite-sources || true
fi
exec gosu app /usr/local/bin/dbackup
