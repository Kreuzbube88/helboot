#!/bin/sh
# Downloads the iPXE bootstrap binaries HELBOOT serves via TFTP
# (ADR-0011). Usage: fetch-ipxe.sh [target-directory]
# Default target: ./assets/tftp
set -eu

TARGET="${1:-./assets/tftp}"
BASE="https://boot.ipxe.org"

mkdir -p "$TARGET"

fetch() {
    src="$1"
    dst="$2"
    echo "fetching $BASE/$src -> $TARGET/$dst"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$TARGET/$dst" "$BASE/$src"
    else
        wget -q -O "$TARGET/$dst" "$BASE/$src"
    fi
}

fetch "undionly.kpxe" "undionly.kpxe"     # BIOS PXE ROMs
fetch "ipxe.efi" "ipxe.efi"               # x86_64 UEFI
fetch "i386-efi/ipxe.efi" "ipxe-i386.efi" # 32-bit UEFI
fetch "arm64-efi/ipxe.efi" "ipxe-arm64.efi" # ARM64 UEFI (e.g. Raspberry Pi)

echo "done: $(ls "$TARGET" | tr '\n' ' ')"
