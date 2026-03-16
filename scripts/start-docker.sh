#!/bin/bash
# Start Docker daemon in WSL (when service/systemctl don't work)
# Run in background: nohup bash scripts/start-docker.sh > /tmp/dockerd.log 2>&1 &

if pgrep -x dockerd > /dev/null; then
  echo "Docker daemon already running"
  exit 0
fi

echo "Starting Docker daemon..."
sudo dockerd > /tmp/dockerd.log 2>&1 &
sleep 5

if docker info > /dev/null 2>&1; then
  echo "Docker is ready"
else
  echo "Docker may still be starting. Check: docker info"
fi
