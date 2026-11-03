#!/bin/sh
set -eu
if [ "${1:-}" = "decrypt" ]; then
  exec gosu app /usr/local/bin/dbackup "$@"
fi
mkdir -p /data/meta /data/backups
chown -R app:app /data
if [ -d /sqlite-sources ] && [ -w /sqlite-sources ]; then
  chown app:app /sqlite-sources || true
fi
exec gosu app /usr/local/bin/dbackup
