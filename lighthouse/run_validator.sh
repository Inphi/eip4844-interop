#!/usr/bin/env bash

#
# Usage: ./validator_client.sh <BEACON-NODE-HTTP> <OPTIONAL-DEBUG-LEVEL>

set -Eeuo pipefail

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
    --validators-dir /data/validators \
    --secrets-dir /data/secrets \
    --testnet-dir /config_data/custom_config_data \
	--init-slashing-protection \
	--beacon-nodes ${@:$OPTIND:1} \
	--suggested-fee-recipient 0x690B9A9E9aa1C9dB991C7721a92d351Db4FaC990
