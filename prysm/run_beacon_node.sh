#!/bin/env bash

set -exu -o pipefail

: "${EXECUTION_RPC:-}"
: "${EXECUTION_NODE_URL:-}"
: "${PROCESS_NAME:-beacon-node}"
: "${TRACING_ENDPOINT:-}"
: "${VERBOSITY:-info}"
: "${P2P_PRIV_KEY:-}"
: "${P2P_TCP_PORT:-13000}"

# wait for the execution node to start
RETRIES=60
i=0
until curl --silent --fail "$EXECUTION_RPC";
do
    sleep 1
    if [ $i -eq $RETRIES ]; then
        echo 'Timed out waiting for execution node'
        exit 1
    fi
    echo 'Waiting for execution node...'
    ((i=i+1))
done

EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)

beacon-chain\
    --accept-terms-of-use \
    --verbosity="$VERBOSITY" \
    --datadir /chaindata \
    --force-clear-db \
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
    --p2p-local-ip 0.0.0.0 \
    --p2p-host-ip "$EXTERNAL_IP" \
    --p2p-tcp-port $P2P_TCP_PORT \
    --p2p-priv-key="$P2P_PRIV_KEY" \
    --suggested-fee-recipient=0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b \
    --subscribe-all-subnets \
    --enable-tracing \
    --tracing-endpoint "$TRACING_ENDPOINT" \
    --tracing-process-name "$PROCESS_NAME" $@
