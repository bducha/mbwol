#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="${IMAGE_NAME:-mbwol-qemu-test}"
NO_BUILD=false
RUNNER_ARGS=()

while [ $# -gt 0 ]; do
    case "$1" in
        --no-build)
            NO_BUILD=true
            shift
            ;;
        *)
            RUNNER_ARGS+=("$1")
            shift
            ;;
    esac
done

if [ "$NO_BUILD" = false ]; then
    echo "Building test image ($IMAGE_NAME)..."
    docker build -t "$IMAGE_NAME" -f "$REPO_ROOT/tests/qemu/Dockerfile" "$REPO_ROOT"
fi

echo "Running QEMU integration test..."
if [ ${#RUNNER_ARGS[@]} -gt 0 ]; then
    docker run --rm "$IMAGE_NAME" "${RUNNER_ARGS[@]}"
else
    docker run --rm "$IMAGE_NAME"
fi
