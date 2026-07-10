#!/bin/sh
# HELBOOT container entrypoint: seed boot assets into the data volume,
# then hand over to the server binary (PID 1 stays the Go supervisor).
set -eu

DATA_DIR="${HELBOOT_DATA_DIR:-/data}"
ASSETS_DIR="${HELBOOT_ASSETS_DIR:-$DATA_DIR/assets}"
TFTP_DIR="$ASSETS_DIR/tftp"

mkdir -p "$TFTP_DIR"

# Copy bundled boot assets on first start only — user-updated files in
# the volume are never overwritten.
seed() {
    src_dir="$1"
    dst_dir="$2"
    for f in "$src_dir"/*; do
        [ -e "$f" ] || continue
        name="$(basename "$f")"
        if [ ! -e "$dst_dir/$name" ]; then
            cp "$f" "$dst_dir/$name"
            echo "seeded boot asset: $name"
        fi
    done
}
seed /app/seed/tftp "$TFTP_DIR"     # iPXE binaries, served via TFTP + /boot/assets/
seed /app/seed/assets "$ASSETS_DIR" # HTTP-only assets (e.g. wimboot)

exec /usr/local/bin/helboot "$@"
