#!/bin/env bash

set -exu -o pipefail

: "${EXECUTION_NODE_URL:-}"
: "${PROCESS_NAME:-beacon-node}"
: "${TRACING_ENDPOINT:-}"
: "${VERBOSITY:-info}"
: "${P2P_TCP_PORT:-13000}"
: "${MIN_SYNC_PEERS:-0}"

EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)

BOOTNODE=$(cat /config_data/custom_config_data/boot_enr.yaml | sed 's/- //')
openssl rand -hex 32 | tr -d '\n' > /tmp/priv-key

beacon-chain\
    --accept-terms-of-use \
    --verbosity="$VERBOSITY" \
    --datadir /chaindata \
    --force-clear-db \
    --bootstrap-node $BOOTNODE \
    --genesis-state=/config_data/custom_config_data/genesis.ssz \
    --execution-endpoint="$EXECUTION_NODE_URL" \
    --jwt-secret=/config_data/cl/jwtsecret \
    --chain-config-file=/config_data/custom_config_data/config.yaml \
    --contract-deployment-block 0 \
    --deposit-contract 0x4242424242424242424242424242424242424242 \
    --rpc-host 0.0.0.0 \
    --rpc-port 4000 \
    --grpc-gateway-host 0.0.0.0 \
    --grpc-gateway-port 3500 \
    --enable-debug-rpc-endpoints \
    --min-sync-peers "$MIN_SYNC_PEERS" \
    --p2p-local-ip 0.0.0.0 \
    --p2p-host-ip "$EXTERNAL_IP" \
    --p2p-tcp-port $P2P_TCP_PORT \
    --p2p-priv-key=/tmp/priv-key \
    --suggested-fee-recipient=0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b \
    --subscribe-all-subnets \
    --enable-tracing \
    --tracing-endpoint "$TRACING_ENDPOINT" \
    --tracing-process-name "$PROCESS_NAME" $@
