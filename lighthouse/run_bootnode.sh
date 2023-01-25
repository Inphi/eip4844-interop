#!/usr/bin/env bash

#
# Generates a bootnode enr and saves it in $TESTNET/boot_enr.yaml
# Starts a bootnode from the generated enr.
#

set -Eeuo pipefail

source /config/values.env

echo "Generating bootnode enr"

EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)
BOOTNODE_PORT=4242

lcli \
	generate-bootnode-enr \
	--ip $EXTERNAL_IP \
	--udp-port $BOOTNODE_PORT \
	--tcp-port $BOOTNODE_PORT \
	--genesis-fork-version $GENESIS_FORK_VERSION \
	--output-dir /data/bootnode

bootnode_enr=`cat /data/bootnode/enr.dat`
echo "- $bootnode_enr" > /config_data/custom_config_data/boot_enr.yaml
# overwrite the static bootnode file too
echo "- $bootnode_enr" > /config_data/custom_config_data/boot_enr.txt

echo "Generated bootnode enr - $bootnode_enr"

DEBUG_LEVEL=${1:-info}

echo "Starting bootnode"

exec lighthouse boot_node \
    --testnet-dir /config_data/custom_config_data \
    --port $BOOTNODE_PORT \
    --listen-address 0.0.0.0 \
    --disable-packet-filter \
    --network-dir /data/bootnode \
