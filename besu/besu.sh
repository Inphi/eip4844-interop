#!/bin/sh

LOG4J_CONFIGURATION_FILE=/opt/besu/log4j2.xml \
/opt/besu/bin/besu \
--data-path=/data \
--genesis-file=/config_data/custom_config_data/besu.json \
--rpc-http-enabled=true \
--rpc-http-host="0.0.0.0" \
--rpc-http-port=8545 \
--engine-rpc-enabled=true \
--engine-rpc-port=8551 \
--engine-host-allowlist="*" \
--rpc-http-cors-origins="*" \
--host-allowlist="*" \
--p2p-enabled=true \
--sync-mode="FULL" \
--data-storage-format="BONSAI" \
--engine-jwt-secret=/config_data/el/jwtsecret
