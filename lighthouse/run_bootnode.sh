#!/usr/bin/env bash

#
# Generates a bootnode enr and saves it in $TESTNET/boot_enr.yaml
# Starts a bootnode from the generated enr.
#

set -Eeuo pipefail

source /config/vars.env

echo "Generating bootnode enr"

EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)

lcli \
	generate-bootnode-enr \
	--ip $EXTERNAL_IP \
	--udp-port $BOOTNODE_PORT \
	--tcp-port $BOOTNODE_PORT \
	--genesis-fork-version $GENESIS_FORK_VERSION \
	--output-dir $DATADIR/bootnode

bootnode_enr=`cat $DATADIR/bootnode/enr.dat`
echo "- $bootnode_enr" > $TESTNET_DIR/boot_enr.yaml

echo "Generated bootnode enr and written to $TESTNET_DIR/boot_enr.yaml"

DEBUG_LEVEL=${1:-info}

echo "Starting bootnode"

exec lighthouse boot_node \
    --testnet-dir $TESTNET_DIR \
    --port $BOOTNODE_PORT \
    --listen-address 0.0.0.0 \
    --disable-packet-filter \
    --network-dir $DATADIR/bootnode \
