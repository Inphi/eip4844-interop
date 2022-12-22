#!/bin/env bash

set -Eeuo pipefail

source /config/vars.env

SUBSCRIBE_ALL_SUBNETS=
DEBUG_LEVEL=${DEBUG_LEVEL:-debug}
EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)

# Get options
while getopts "d:sh" flag; do
  case "${flag}" in
    d) DEBUG_LEVEL=${OPTARG};;
    s) SUBSCRIBE_ALL_SUBNETS="--subscribe-all-subnets";;
    h)
       echo "Start a beacon node"
       echo
       echo "usage: $0 <Options> <DATADIR> <EXECUTION_ENDPOINT>"
       echo
       echo "Options:"
       echo "   -s: pass --subscribe-all-subnets to 'lighthouse bn ...', default is not passed"
       echo "   -d: DEBUG_LEVEL, default info"
       echo "   -h: this help"
       echo
       echo "Positional arguments:"
       echo "  DATADIR             Value for --datadir parameter"
       echo "  EXECUTION_ENDPOINT  Value for --execution-endpoint parameter"
       exit
       ;;
  esac
done

set -x

# Get positional arguments
data_dir=${@:$OPTIND+0:1}
execution_endpoint=${@:$OPTIND+1:1}
network_port=$P2P_PORT
http_port=8000

exec lighthouse \
    --debug-level $DEBUG_LEVEL \
    bn \
    $SUBSCRIBE_ALL_SUBNETS \
    --datadir $data_dir \
    --testnet-dir $TESTNET_DIR \
    --enable-private-discovery \
    --staking \
    --enr-address $EXTERNAL_IP \
    --enr-udp-port $network_port \
    --enr-tcp-port $network_port \
    --port $network_port \
    --http-address 0.0.0.0 \
    --http-port $http_port \
    --disable-packet-filter \
    --target-peers $((BN_COUNT)) \
    --execution-endpoint $execution_endpoint \
    --trusted-setup-file /config/trusted_setup.txt \
    --execution-jwt /config/jwtsecret
