#!/bin/bash
# build.sh – baut das homeport Container-Image mit Podman
# Usage: ./build.sh [tag]   default: latest
set -euo pipefail

TAG="${1:-latest}"
IMAGE="localhost/homeport:${TAG}"

echo "Building ${IMAGE}..."
podman build -t "${IMAGE}" .

echo ""
echo "Image size:"
podman images --format "{{.Repository}}:{{.Tag}} {{.Size}}" "${IMAGE}"

echo ""
echo "Deploy: cp deploy/homeport.container ~/.config/containers/systemd/"
echo "        systemctl --user daemon-reload && systemctl --user start homeport"
