#!/bin/bash
#
# Suggested deploy script for pasta
#

CONTAINER_NAME="pasta"           # Name of the container
CONTAINER_MEMORY="64"           # Memory limitations for the container in MB
DATA="/srv/pasta"                # Data directory for config file and pastas
PORT="8199"                      # Exposed port of the container


function usage() {
	echo "$0 - Simple pasta deployment script"
	echo "Usage: $0 [DATADIR]         - deploy instance as docker container"
	echo "       $0 --rm              - remove container"
	echo ""
	echo "OPTIONS"
	echo "   DATADIR    - Directory where data will be stored"
}

if [[ $# -ge 1 ]]; then
	DATA="$1"
	if [[ $DATA == "-h" || $DATA == "--help" ]]; then
		usage
		exit 0
	fi
fi



if [[ $CONTAINER_NAME != "" ]]; then
	docker container stop "$CONTAINER_NAME"
	docker container rm "$CONTAINER_NAME"
fi

# Special use case: Remove container
if [[ $DATA == "--rm" ]]; then
	exit 0
fi

docker pull grisu48/pasta


# Prepare data directory
mkdir -p "$DATA"
if [[ ! -s "$DATA/pastad.toml" ]]; then
	echo "Using default configuration file $DATA/pastad.toml"
	cp "pastad.toml.example" "$DATA/pastad.toml"
else
	echo "Found existing pastad.toml configuration"
fi

set -e

docker container create --name "$CONTAINER_NAME" -p "$PORT":8199 -v "$DATA" grisu48/pasta
docker update --restart unless-stopped "$CONTAINER_NAME"
if [[ "$CONTAINER_MEMORY" != "" ]]; then
	# Update memory, use double the memory as swap
	docker update --memory "${CONTAINER_MEMORY}M" --memory-swap "$(($CONTAINER_MEMORY * 2))M" "$CONTAINER_NAME"
fi
docker container start "$CONTAINER_NAME"
