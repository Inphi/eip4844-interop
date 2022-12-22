#!/bin/env bash

set -exu -o pipefail

: "${BEACON_NODE_RPC:-}"
: "${TESTNET_DIR:-}"

# wait for the beacon node to start
RETRIES=60
i=0
until curl --fail --silent "${BEACON_NODE_RPC}/eth/v1/node/version"; do
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
P2P_ADDRESS=$(curl --fail "$BEACON_NODE_RPC"/eth/v1/node/identity | jq -r .data.p2p_addresses[0])

if [ "$PEER" = "null" ]; then
    echo "Unable to start beacon-node: Beacon Node address is unavailable via $BEACON_NODE_RPC}"
    exit 1
fi

echo "Retrieving genesis state from beacon node..."
curl --header "Accept: application/octet-stream" --output $TESTNET_DIR/genesis.ssz "${BEACON_NODE_RPC}/eth/v2/debug/beacon/states/genesis"
echo "Genesis state saved to $TESTNET_DIR/genesis.ssz"

run_beacon_node.sh \
    --boot-nodes "$PEER" \
    --libp2p-addresses "$P2P_ADDRESS" $@
