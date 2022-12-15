#!/usr/bin/env bash

#
# Usage: ./validator_client.sh <BEACON-NODE-HTTP> <OPTIONAL-DEBUG-LEVEL>

set -Eeuo pipefail

source /config/vars.env

DEBUG_LEVEL=info

BUILDER_PROPOSALS=

# Get options
while getopts "pd:" flag; do
  case "${flag}" in
    p) BUILDER_PROPOSALS="--builder-proposals";;
    d) DEBUG_LEVEL=${OPTARG};;
  esac
done

exec lighthouse \
	--debug-level $DEBUG_LEVEL \
	vc \
	$BUILDER_PROPOSALS \
	--datadir $DATADIR/node_1 \
	--testnet-dir $TESTNET_DIR \
	--init-slashing-protection \
	--beacon-nodes ${@:$OPTIND:1} \
	--suggested-fee-recipient 0x690B9A9E9aa1C9dB991C7721a92d351Db4FaC990 \
	$VC_ARGS
