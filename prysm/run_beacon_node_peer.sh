#!/bin/env bash

set -exu -o pipefail

: "${BEACON_NODE_RPC:-}"

# wait for the beacon node to start
RETRIES=60
i=0
until curl --fail --silent "${BEACON_NODE_RPC}/eth/v1/node/health"; do
    sleep 1
    if [ $i -eq $RETRIES ]; then
        echo "Timed out retrieving beacon node p2p addr"
        exit 1
    fi
    echo "waiting for beacon node RPC..."
    ((i=i+1))
done

# Retrieve the enr of the bootstrap node. We will sync blocks from this peer.
PEER=$(curl --fail "$BEACON_NODE_RPC"/eth/v1/node/identity | jq '.data.enr' | tr -d '"' | tr -d '=')
# Retrieve the generated genesis time so we can follow the peer with matching state roots
INTEROP_GENESIS_TIME=$(curl --fail "$BEACON_NODE_RPC"/eth/v1/beacon/genesis | jq .data.genesis_time | tr -d '"')

if [ "$PEER" = "null" ]; then
    echo "Unable to start beacon-node: Beacon Node address is unavailable via $BEACON_NODE_RPC}"
    exit 1
fi

run_beacon_node.sh \
    --min-sync-peers=1 \
    --bootstrap-node "$PEER" \
    --interop-genesis-time "$INTEROP_GENESIS_TIME" $@
