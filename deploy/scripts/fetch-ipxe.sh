#!/bin/sh
# Downloads the boot assets HELBOOT ships in the image (ADR-0011):
# iPXE bootstrap binaries served via raw TFTP, plus HTTP-only assets
# (currently: wimboot). Usage: fetch-ipxe.sh [target-directory]
# Default target: ./assets
#
# Writes two subdirectories under the target:
#   tftp/  iPXE binaries, served by the TFTP server and via /boot/assets/
#   http/  assets only ever fetched over HTTP by the generated iPXE
#          script (e.g. wimboot), served from the assets root
#
# Upstream (boot.ipxe.org, GitHub releases) occasionally reorganizes its
# artifact layout, so every artifact has a list of candidate paths and a
# missing artifact is a WARNING, not a failure: the server logs a clear
# hint at runtime when a client requests a binary that is absent, and
# users can drop their own builds into the target directory at any time.
set -u

TARGET="${1:-./assets}"
TFTP_DIR="$TARGET/tftp"
HTTP_DIR="$TARGET/http"
BASE="https://boot.ipxe.org"
MISSING=0

mkdir -p "$TFTP_DIR" "$HTTP_DIR"

download() {
    url="$1"
    out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$out" "$url"
    else
        wget -q -O "$out" "$url"
    fi
}

# fetch <dir> <destination> <candidate-path-or-url>...
# A candidate starting with http:// or https:// is used as-is; otherwise
# it is resolved relative to $BASE.
fetch() {
    dir="$1"
    dst="$2"
    shift 2
    for src in "$@"; do
        case "$src" in
            http://* | https://*) url="$src" ;;
            *) url="$BASE/$src" ;;
        esac
        if download "$url" "$dir/$dst"; then
            echo "fetched $url -> $dir/$dst"
            return 0
        fi
        rm -f "$dir/$dst"
    done
    echo "WARNING: could not fetch $dst (tried: $*)" >&2
    MISSING=$((MISSING + 1))
    return 0
}

fetch "$TFTP_DIR" "undionly.kpxe" "undionly.kpxe"                              # BIOS PXE ROMs
fetch "$TFTP_DIR" "ipxe.efi" "x86_64-efi/ipxe.efi" "ipxe.efi" "ipxe-x86_64.efi" # x86_64 UEFI
fetch "$TFTP_DIR" "ipxe-i386.efi" "i386-efi/ipxe.efi" "ipxe-i386.efi"           # 32-bit UEFI
fetch "$TFTP_DIR" "ipxe-arm64.efi" "arm64-efi/ipxe.efi" "arm64-efi/snponly.efi" # ARM64 UEFI

# Boot media for machines without PXE (§21): the user writes these to a
# CD/USB stick; the stick boots iPXE, which then reaches HELBOOT over
# the network exactly like a PXE client.
fetch "$TFTP_DIR" "ipxe.iso" "ipxe.iso" "x86_64-exp/ipxe.iso" # bootable CD/DVD image
fetch "$TFTP_DIR" "ipxe.usb" "ipxe.usb" "x86_64-exp/ipxe.usb" # raw USB disk image

# wimboot chainloads Windows PE for Windows providers (kernel: wimboot in
# provider manifests); fetched from the upstream ipxe/wimboot releases,
# always the latest build, no version pinning (matching the iPXE binaries
# above).
fetch "$HTTP_DIR" "wimboot" "https://github.com/ipxe/wimboot/releases/latest/download/wimboot"

echo "done: tftp=[$(ls "$TFTP_DIR" 2>/dev/null | tr '\n' ' ')] http=[$(ls "$HTTP_DIR" 2>/dev/null | tr '\n' ' ')]"
if [ "$MISSING" -gt 0 ]; then
    echo "note: $MISSING artifact(s) missing; place your own builds in $TFTP_DIR or $HTTP_DIR" >&2
fi
exit 0
