#!/bin/sh
# HELBOOT container entrypoint: seed boot assets into the data volume,
# then hand over to the server binary (PID 1 stays the Go supervisor).
set -eu

DATA_DIR="${HELBOOT_DATA_DIR:-/data}"
TFTP_DIR="${HELBOOT_ASSETS_DIR:-$DATA_DIR/assets}/tftp"

mkdir -p "$TFTP_DIR"
# Copy bundled iPXE binaries on first start only — user-updated binaries
# in the volume are never overwritten.
for f in /app/seed/tftp/*; do
    [ -e "$f" ] || continue
    name="$(basename "$f")"
    if [ ! -e "$TFTP_DIR/$name" ]; then
        cp "$f" "$TFTP_DIR/$name"
        echo "seeded boot asset: $name"
    fi
done

exec /usr/local/bin/helboot "$@"
