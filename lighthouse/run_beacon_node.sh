#!/bin/env bash

set -exu -o pipefail

: "${EXECUTION_NODE_URL:-}"
: "${VERBOSITY:-info}"

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
NETWORK_PORT=9000
HTTP_PORT=5052

lighthouse \
	beacon_node \
	--debug-level $VERBOSITY \
	--datadir /chaindata \
	--purge-db \
	--eth1-endpoints "$EXECUTION_NODE_URL" \
	--testnet-dir $TESTNET_DIR \
	--port $NETWORK_PORT \
	--http \
	--http-port $HTTP_PORT \
	--enable-private-discovery \
	--enr-address $EXTERNAL_IP \
	--enr-udp-port $NETWORK_PORT \
	--enr-tcp-port $NETWORK_PORT \
	--subscribe-all-subnets \
	--disable-packet-filter $@
