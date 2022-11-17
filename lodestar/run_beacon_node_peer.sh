#!/bin/sh

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

echo "Connected to beacon node REST API ${BEACON_NODE_RPC}/eth/v1/node/health"

# Retrieve the enr of the bootstrap node. We will sync blocks from this peer.
ENR=$(curl --fail "$BEACON_NODE_RPC"/eth/v1/node/identity | jq '.data.enr' | tr -d '"' | tr -d '=')

# Retrieve the generated genesis time so we can follow the peer with matching state roots
INTEROP_GENESIS_TIME=$(curl --fail "$BEACON_NODE_RPC"/eth/v1/beacon/genesis | jq .data.genesis_time | tr -d '"')

if [ "$ENR" = "null" ]; then
    echo "Unable to start beacon-node: Beacon Node address is unavailable via $BEACON_NODE_RPC}"
    exit 1
fi

# https://chainsafe.notion.site/chainsafe/Lodestar-flags-02406e481f664d84adb56c2c348e49aa
./lodestar dev \
    --paramsFile /config/chain-config.yml \
    --genesisValidators 1 \
    --genesisTime "$INTEROP_GENESIS_TIME" \
    --dataDir /chaindata \
    --network.connectToDiscv5Bootnodes true \
    --bootnodes $ENR \
    --eth1 \
    --eth1.providerUrls="$EXECUTION_NODE_URL" \
    --execution.urls="$EXECUTION_NODE_URL" \
    --reset \
    --server http://localhost:3500 \
    --rest \
    --rest.address 0.0.0.0 \
    --rest.port 3500 \
    --rest.namespace "*" \
