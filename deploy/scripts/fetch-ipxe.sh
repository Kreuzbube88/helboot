#!/bin/sh
# Downloads the iPXE bootstrap binaries HELBOOT serves via TFTP and the
# writable boot media (ADR-0011). Usage: fetch-ipxe.sh [target-directory]
# Default target: ./assets/tftp
#
# Upstream (boot.ipxe.org) occasionally reorganizes its artifact layout,
# so every artifact has a list of candidate paths and a missing artifact
# is a WARNING, not a failure: the server logs a clear hint at runtime
# when a client requests a binary that is absent, and users can drop
# their own builds into the target directory at any time.
set -u

TARGET="${1:-./assets/tftp}"
BASE="https://boot.ipxe.org"
MISSING=0

mkdir -p "$TARGET"

download() {
    url="$1"
    out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$out" "$url"
    else
        wget -q -O "$out" "$url"
    fi
}

# fetch <destination> <candidate-path>...
fetch() {
    dst="$1"
    shift
    for src in "$@"; do
        if download "$BASE/$src" "$TARGET/$dst"; then
            echo "fetched $BASE/$src -> $TARGET/$dst"
            return 0
        fi
        rm -f "$TARGET/$dst"
    done
    echo "WARNING: could not fetch $dst (tried: $*)" >&2
    MISSING=$((MISSING + 1))
    return 0
}

fetch "undionly.kpxe" "undionly.kpxe"                              # BIOS PXE ROMs
fetch "ipxe.efi" "x86_64-efi/ipxe.efi" "ipxe.efi" "ipxe-x86_64.efi" # x86_64 UEFI
fetch "ipxe-i386.efi" "i386-efi/ipxe.efi" "ipxe-i386.efi"           # 32-bit UEFI
fetch "ipxe-arm64.efi" "arm64-efi/ipxe.efi" "arm64-efi/snponly.efi" # ARM64 UEFI

# Boot media for machines without PXE (§21): the user writes these to a
# CD/USB stick; the stick boots iPXE, which then reaches HELBOOT over
# the network exactly like a PXE client.
fetch "ipxe.iso" "ipxe.iso" "x86_64-exp/ipxe.iso" # bootable CD/DVD image
fetch "ipxe.usb" "ipxe.usb" "x86_64-exp/ipxe.usb" # raw USB disk image

echo "done: $(ls "$TARGET" 2>/dev/null | tr '\n' ' ')"
if [ "$MISSING" -gt 0 ]; then
    echo "note: $MISSING artifact(s) missing; place your own iPXE builds in $TARGET" >&2
fi
exit 0
