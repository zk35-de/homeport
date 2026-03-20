#!/bin/bash
set -e
echo "Building container image..."
podman build -t localhost/homeport:latest .
echo ""
echo "Image size:"
podman images --format "{{.Repository}}:{{.Tag}} {{.Size}}" localhost/homeport:latest
