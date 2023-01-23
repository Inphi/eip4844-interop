#!/bin/env bash

set -exu -o pipefail

source /shared/generated-shared.env

: "${EXECUTION_NODE_URL:-}"
: "${PROCESS_NAME:-beacon-node}"
: "${TRACING_ENDPOINT:-}"
: "${VERBOSITY:-info}"
: "${P2P_PRIV_KEY:-}"
: "${P2P_TCP_PORT:-13000}"

EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)

beacon-chain \
    --accept-terms-of-use \
    --verbosity="${VERBOSITY}" \
    --datadir /chaindata \
    --force-clear-db \
    --interop-num-validators 4 \
    --interop-eth1data-votes \
    --interop-genesis-state /shared/generated-genesis.ssz \
    --interop-genesis-time ${GENESIS} \
    --execution-endpoint="$EXECUTION_NODE_URL" \
    --jwt-secret=/shared/jwtsecret \
    --chain-config-file=/shared/chain-config.yml \
    --contract-deployment-block 0 \
    --deposit-contract 0x8A04d14125D0FDCDc742F4A05C051De07232EDa4 \
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
