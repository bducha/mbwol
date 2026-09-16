#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Configuration
SERVER_IP="${SERVER_IP:-}"
HTTP_PORT="${HTTP_PORT:-8000}"
WOL_PORT="${WOL_PORT:-9}"
MAC_ADDR="${MAC_ADDR:-52:54:00:12:34:56}"
HOST_ID="${HOST_ID:-node1}"
CONFIG_NAME="${CONFIG_NAME:-target-os}"
BOOT_ENTRY="${BOOT_ENTRY:-1}"
TIMEOUT="${TIMEOUT:-30}"
OVMF_PATH="${OVMF_PATH:-}"
MBWOL_BIN="${MBWOL_BIN:-}"

while [ $# -gt 0 ]; do
    case "$1" in
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --boot-entry)
            BOOT_ENTRY="$2"
            shift 2
            ;;
        --server-ip)
            SERVER_IP="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --timeout <sec>      Test timeout in seconds (default: 30)"
            echo "  --boot-entry <entry> Expected boot entry number (default: 1)"
            echo "  --server-ip <ip>     Server IP address (default: auto-detected)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# Locate mbwol binary
if [ -n "$MBWOL_BIN" ] && [ -x "$MBWOL_BIN" ]; then
    :
elif [ -x "/usr/local/bin/mbwol" ]; then
    MBWOL_BIN="/usr/local/bin/mbwol"
elif [ -x "$REPO_ROOT/mbwol" ]; then
    MBWOL_BIN="$REPO_ROOT/mbwol"
elif which mbwol >/dev/null 2>&1; then
    MBWOL_BIN="$(which mbwol)"
else
    echo "Error: mbwol binary not found." >&2
    exit 1
fi

# Locate OVMF firmware
if [ -z "$OVMF_PATH" ]; then
    for candidate in \
        /usr/share/ovmf/OVMF.fd \
        /usr/share/OVMF/OVMF_CODE.fd \
        /usr/share/OVMF/OVMF.fd \
        /usr/share/edk2-ovmf/x64/OVMF.fd; do
        if [ -f "$candidate" ]; then
            OVMF_PATH="$candidate"
            break
        fi
    done
fi

if [ -z "$OVMF_PATH" ] || [ ! -f "$OVMF_PATH" ]; then
    echo "Error: OVMF firmware not found." >&2
    exit 1
fi

# Determine server IP
if [ -z "$SERVER_IP" ]; then
    SERVER_IP=$(ip -4 addr show eth0 2>/dev/null | awk '/inet / {print $2}' | cut -d/ -f1 || true)
    if [ -z "$SERVER_IP" ]; then
        SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
    fi
    if [ -z "$SERVER_IP" ]; then
        SERVER_IP="127.0.0.1"
    fi
fi

TMP_DIR=$(mktemp -d /tmp/mbwol-qemu-test.XXXXXX)

MBWOL_PID=""
WOL_PID=""
QEMU_PID=""

cleanup() {
    local exit_code=$?
    if [ -n "$QEMU_PID" ] && kill -0 "$QEMU_PID" 2>/dev/null; then
        kill -9 "$QEMU_PID" 2>/dev/null || true
        wait "$QEMU_PID" 2>/dev/null || true
    fi
    if [ -n "$WOL_PID" ] && kill -0 "$WOL_PID" 2>/dev/null; then
        kill -9 "$WOL_PID" 2>/dev/null || true
        wait "$WOL_PID" 2>/dev/null || true
    fi
    if [ -n "$MBWOL_PID" ] && kill -0 "$MBWOL_PID" 2>/dev/null; then
        kill -9 "$MBWOL_PID" 2>/dev/null || true
        wait "$MBWOL_PID" 2>/dev/null || true
    fi
    rm -rf "$TMP_DIR"
    exit "$exit_code"
}
trap cleanup EXIT INT TERM

echo "Using server IP: $SERVER_IP"
echo "Using mbwol binary: $MBWOL_BIN"
echo "Using OVMF firmware: $OVMF_PATH"

# Write mbwol configuration
cat << EOF > "$TMP_DIR/mbwol.json"
{
  "hosts": {
    "$HOST_ID": {
      "id": "$HOST_ID",
      "ip": "$SERVER_IP",
      "macAddress": "$MAC_ADDR",
      "broadcastIp": "255.255.255.255",
      "configs": {
        "$CONFIG_NAME": "set default=$BOOT_ENTRY\n"
      },
      "timeout": 60,
      "resetAfterGet": false
    }
  }
}
EOF

# Prepare GRUB configuration
sed -e "s/<server-ip>/$SERVER_IP/g" "$SCRIPT_DIR/grub.cfg" > "$TMP_DIR/grub.cfg"

# Generate standalone GRUB EFI binary
grub-mkstandalone \
    -O x86_64-efi \
    -o "$TMP_DIR/bootx64.efi" \
    --modules="net efinet tftp" \
    "/boot/grub/grub.cfg=$TMP_DIR/grub.cfg"

# Create FAT EFI system partition image
dd if=/dev/zero of="$TMP_DIR/esp.img" bs=1M count=64 status=none
mkfs.vfat "$TMP_DIR/esp.img" >/dev/null
mmd -i "$TMP_DIR/esp.img" ::EFI
mmd -i "$TMP_DIR/esp.img" ::EFI/BOOT
mcopy -i "$TMP_DIR/esp.img" "$TMP_DIR/bootx64.efi" ::EFI/BOOT/BOOTX64.EFI

# Start mbwol server
MBWOL_CONFIG_FILE="$TMP_DIR/mbwol.json" \
MBWOL_HTTP_PORT="$HTTP_PORT" \
MBWOL_VERBOSE=true \
"$MBWOL_BIN" > "$TMP_DIR/mbwol.log" 2>&1 &
MBWOL_PID=$!

# Wait for mbwol HTTP API
API_READY=false
for _ in $(seq 1 50); do
    if curl -s -o /dev/null "http://127.0.0.1:$HTTP_PORT/" 2>/dev/null; then
        API_READY=true
        break
    fi
    sleep 0.1
done

if [ "$API_READY" = false ]; then
    echo "Error: mbwol server failed to start." >&2
    cat "$TMP_DIR/mbwol.log" >&2
    exit 1
fi
echo "mbwol server started (PID $MBWOL_PID)"

# Start Wake-on-LAN listener
python3 -u -c '
import socket
import sys

mac_str = sys.argv[1]
port = int(sys.argv[2])
timeout = float(sys.argv[3])
ready_file = sys.argv[4]

mac = bytes.fromhex(mac_str.replace(":", ""))
expected_magic = b"\xff" * 6 + mac * 16

sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
sock.bind(("0.0.0.0", port))
sock.settimeout(timeout)

with open(ready_file, "w") as f:
    f.write("READY\n")

try:
    data, addr = sock.recvfrom(1024)
    if data == expected_magic:
        print(f"WoL packet verified from {addr}")
        sys.exit(0)
    else:
        print(f"Invalid WoL packet received: {data.hex()}", file=sys.stderr)
        sys.exit(1)
except socket.timeout:
    print("Timed out waiting for WoL packet", file=sys.stderr)
    sys.exit(1)
' "$MAC_ADDR" "$WOL_PORT" "10.0" "$TMP_DIR/wol_ready" > "$TMP_DIR/wol.log" 2>&1 &
WOL_PID=$!

# Wait for WoL listener socket to bind
for _ in $(seq 1 50); do
    if [ -f "$TMP_DIR/wol_ready" ]; then
        break
    fi
    sleep 0.1
done

# Trigger boot request via HTTP
echo "Sending boot request: POST /boot/$HOST_ID/$CONFIG_NAME"
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://127.0.0.1:$HTTP_PORT/boot/$HOST_ID/$CONFIG_NAME")
if [ "$HTTP_STATUS" != "200" ]; then
    echo "Error: boot request returned HTTP $HTTP_STATUS" >&2
    cat "$TMP_DIR/mbwol.log" >&2
    exit 1
fi

# Verify Wake-on-LAN packet capture
if ! wait "$WOL_PID"; then
    echo "Error: WoL packet verification failed." >&2
    cat "$TMP_DIR/wol.log" >&2
    exit 1
fi
echo "Wake-on-LAN magic packet captured and verified"

# Start QEMU headlessly
echo "Starting QEMU..."
qemu-system-x86_64 \
    -nographic \
    -bios "$OVMF_PATH" \
    -drive file="$TMP_DIR/esp.img",format=raw,if=ide \
    -netdev user,id=net0 \
    -device virtio-net-pci,netdev=net0 > "$TMP_DIR/serial.log" 2>&1 &
QEMU_PID=$!

EXPECTED_MARKER="BOOT_ENTRY=$BOOT_ENTRY"
START_TIME=$(date +%s)
SUCCESS=false

echo "Waiting for '$EXPECTED_MARKER' in serial output (timeout: ${TIMEOUT}s)..."
while true; do
    if grep -Fq "$EXPECTED_MARKER" "$TMP_DIR/serial.log" 2>/dev/null; then
        SUCCESS=true
        break
    fi

    if ! kill -0 "$QEMU_PID" 2>/dev/null; then
        # QEMU process terminated prematurely
        break
    fi

    NOW=$(date +%s)
    if [ $((NOW - START_TIME)) -ge "$TIMEOUT" ]; then
        break
    fi
    sleep 0.5
done

if [ "$SUCCESS" = true ]; then
    echo "Test passed: '$EXPECTED_MARKER' observed in serial console output."
    exit 0
else
    echo "Test failed: marker '$EXPECTED_MARKER' not found." >&2
    echo "--- QEMU Serial Output ---" >&2
    cat "$TMP_DIR/serial.log" >&2
    echo "--- mbwol Server Output ---" >&2
    cat "$TMP_DIR/mbwol.log" >&2
    exit 1
fi
