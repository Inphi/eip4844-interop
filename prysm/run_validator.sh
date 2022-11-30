#!/bin/env bash

# adapated from the hive prysm validator runscript

set -e

mkdir -p /data/vc
mkdir -p /data/validators
echo walletpassword > /wallet.pass

for keystore_path in /data/keystores/*
do
  pubkey=$(basename "$keystore_path")

  validator accounts import \
    --accept-terms-of-use=true \
    --wallet-dir="/data/validators" \
    --keys-dir="/data/keystores/$pubkey" \
    --account-password-file="/data/secrets/$pubkey" \
    --wallet-password-file="/wallet.pass"
done

echo Starting Prysm Validator Client

#--force-clear-db
validator \
    --accept-terms-of-use=true \
    --beacon-rpc-provider=prysm-beacon-node:4000 \
    --rpc \
    --grpc-gateway-host 0.0.0.0 \
    --grpc-gateway-port 7500 \
    --datadir="/data/vc" \
    --wallet-dir="/data/validators" \
    --wallet-password-file="/wallet.pass" \
    --chain-config-file="/config/chain-config.yml"\
    --suggested-fee-recipient 0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b\
    --enable-tracing\
    --tracing-endpoint http://jaeger-tracing:14268/api/traces\
    --tracing-process-name validator-node
