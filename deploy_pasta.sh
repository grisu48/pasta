#!/bin/bash
#
# Suggested deploy script for pasta
#

CONTAINER_NAME="pasta"           # Name of the container
CONTAINER_MEMORY="64"           # Memory limitations for the container in MB
DATA="/srv/pasta"                # Data directory for config file and pastas
PORT="8199"                      # Exposed port of the container





if [[ $CONTAINER_NAME != "" ]]; then
	docker container stop "$CONTAINER_NAME"
	docker container rm "$CONTAINER_NAME"
fi
docker pull grisu48/pasta

set -e

docker container create --name "$CONTAINER_NAME" -p "$PORT":8199 -v "$DATA" grisu48/pasta
docker update --restart unless-stopped "$CONTAINER_NAME"
if [[ "$CONTAINER_MEMORY" != "" ]]; then
	# Update memory, use double the memory as swap
	docker update --memory "${CONTAINER_MEMORY}M" --memory-swap "$(($CONTAINER_MEMORY * 2))M" "$CONTAINER_NAME"
fi
docker container start "$CONTAINER_NAME"
