#!/bin/bash
# Start MySQL without Docker Compose (uses plain docker run)
# Run from project root: bash scripts/start-mysql.sh

CONTAINER="jukespotify-mysql"

if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
  if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    echo "MySQL already running (container: $CONTAINER)"
  else
    echo "Starting existing container..."
    docker start $CONTAINER
  fi
else
  echo "Creating and starting MySQL container..."
  docker run -d \
    --name $CONTAINER \
    -p 3306:3306 \
    -e MYSQL_ROOT_PASSWORD=jukespotify \
    -e MYSQL_DATABASE=jukespotify \
    -v jukespotify_mysql_data:/var/lib/mysql \
    mysql:8
fi

echo "Waiting for MySQL to be ready..."
sleep 10
echo "MySQL should be ready. Run: cd server && go run ."
