#!/bin/sh
set -e

# Fix volume permissions: chown /app/data to homeport if owned by another user.
# This handles the common case where Docker creates the host directory as root.
DATA_DIR="${HOMEPORT_DB:-/app/data/homeport.db}"
DATA_DIR="$(dirname "$DATA_DIR")"

if [ "$(stat -c '%u' "$DATA_DIR")" != "1000" ]; then
    chown -R homeport:homeport "$DATA_DIR"
fi

exec su-exec homeport /app/homeport "$@"
