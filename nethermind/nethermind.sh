#!/usr/bin/env bash

set -x

./Nethermind.Runner \
  --datadir="/db" \
  --Init.ChainSpecPath="/config_data/custom_config_data/chainspec.json" \
  --Init.WebSocketsEnabled=true \
  --JsonRpc.Enabled=true \
  --JsonRpc.EnabledModules="net,eth,consensus,subscribe,web3,admin,txpool,debug,trace" \
  --JsonRpc.EngineEnabledModules="net,eth,consensus,subscribe,web3,admin,txpool,debug,trace" \
  --JsonRpc.Port=8545 \
  --JsonRpc.WebSocketsPort=8546 \
  --JsonRpc.EnginePort=8551 \
  --JsonRpc.Host=0.0.0.0 \
  --JsonRpc.EngineHost=0.0.0.0 \
  --Network.DiscoveryPort=30303 \
  --Network.P2PPort=30303 \
  --Merge.SecondsPerSlot=3 \
  --Init.IsMining=true \
  --JsonRpc.JwtSecretFile=/config_data/el/jwtsecret \
  --Sync.FastSync=false \
  --JsonRpc.MaxBatchSize=1000 \
  --JsonRpc.MaxBatchResponseBodySize=1000000000 \
  --config none.cfg
