#!/usr/bin/env bash

set -exu -o pipefail

: "${EXECUTION_NODE_URL:-}"
: "${PROCESS_NAME:-beacon-node}"
: "${TRACING_ENDPOINT:-}"
: "${VERBOSITY:-info}"
: "${P2P_PRIV_KEY:-}"

# wait for the execution node to start
RETRIES=60
i=0
until curl --silent --fail "$EXECUTION_NODE_URL";
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

beacon-node \
    --accept-terms-of-use \
    --verbosity="$VERBOSITY" \
    --datadir /chaindata \
    --force-clear-db \
    --interop-eth1data-votes \
    --execution-endpoint="$EXECUTION_NODE_URL" \
    --jwt-secret=/config/jwtsecret \
    --deposit-contract 0x4242424242424242424242424242424242424242 \
    --chain-config-file=/config/chain-config.yml \
    --contract-deployment-block 0 \
    --interop-num-validators 4 \
    --rpc-host 0.0.0.0 \
    --rpc-port 4000 \
    --grpc-gateway-host 0.0.0.0 \
    --grpc-gateway-port 3500 \
    --enable-debug-rpc-endpoints \
    --p2p-local-ip 0.0.0.0 \
    --p2p-host-ip "$EXTERNAL_IP" \
    --p2p-priv-key="$P2P_PRIV_KEY"\
    --subscribe-all-subnets \
    --enable-tracing \
    --tracing-endpoint "$TRACING_ENDPOINT" \
    --log-file=execution.log \
    --tracing-process-name "$PROCESS_NAME" $@
