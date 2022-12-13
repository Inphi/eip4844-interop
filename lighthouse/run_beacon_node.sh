#!/bin/env bash

set -exu -o pipefail

: "${EXECUTION_NODE_URL:-}"
: "${VERBOSITY:-info}"
: "${GENERATE_GENESIS:-false}"

DATADIR=/chaindata
VALIDATOR_COUNT=4

GENESIS_TIME=`date +%s`

if [ "$GENERATE_GENESIS" == "true" ] && [ ! -d $DATADIR/validators ]; then
  lcli \
  	insecure-validators \
  	--count $VALIDATOR_COUNT \
  	--base-dir $DATADIR \

  echo Validators generated with keystore passwords at $DATADIR.

	echo "Waiting for genesis state to be created... (this might take a while)"
	RETRIES=60
	i=0
	until [ -f /genesis_data/genesis.ssz ];
	do
			sleep 1
			if [ $i -eq $RETRIES ]; then
					echo 'Timed out waiting for genesis state'
					exit 1
			fi
			echo 'Waiting for genesis state...'
			((i=i+1))
	done

	cp /genesis_data/genesis.ssz $TESTNET_DIR
fi


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
	--debug-level info \
	--datadir "$DATADIR" \
	--purge-db \
	--execution-endpoint "$EXECUTION_NODE_URL"  \
	--execution-jwt-secret-key 0x98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4 \
	--testnet-dir $TESTNET_DIR \
	--port $NETWORK_PORT \
	--http \
	--http-port $HTTP_PORT \
	--http-address 0.0.0.0 \
	--http-allow-sync-stalled \
	--enable-private-discovery \
	--enr-address $EXTERNAL_IP \
	--enr-udp-port $NETWORK_PORT \
	--enr-tcp-port $NETWORK_PORT \
	--disable-enr-auto-update \
	--subscribe-all-subnets \
	--trusted-setup-file $TESTNET_DIR/trusted_setup.txt \
	--disable-packet-filter $@
