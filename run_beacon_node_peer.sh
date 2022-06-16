#!/bin/env bash

set -exu

: "${BEACON_NODE_RPC:-}"

# Give the primary beacon node a sec to boot up. TODO(inphi): this should be more robust
sleep 3

# Retrieve the multiaddr of the primary beacon node. We will sync blocks from this peer.
eval PEER=$(curl --fail --silent "$BEACON_NODE_RPC"/eth/v1/node/identity | jq '.data.p2p_addresses[0]')

# Retrieve the generated genesis time so we can follow the peer with matching state roots
eval INTEROP_GENESIS_TIME=$(curl --fail --silent "$BEACON_NODE_RPC"/eth/v1/beacon/genesis | jq .data.genesis_time)

if [ "$PEER" = "null" ]; then
    echo "Unable to start beacon-node: Beacon Node address is unavailable via $BEACON_NODE_RPC}"
    exit 1
fi

run_beacon_node.sh \
    --min-sync-peers=1 \
    --peer "$PEER" \
    --interop-genesis-time "$INTEROP_GENESIS_TIME" $@
