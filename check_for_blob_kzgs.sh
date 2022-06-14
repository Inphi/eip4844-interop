#!/bin/sh

for i in {18..22}; do
  KZGS=$(curl -s http://localhost:3500/eth/v2/beacon/blocks/${i} | jq '.data.message.body.blob_kzgs[0]')
  if [ "$KZGS" = '"0xc00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"' ]; then
    echo "KZGs verified!"
    exit 0
  fi
  if [ "$KZGS" != 'null' ]; then
    echo "Unexpected KZGS: $KZGS"
    exit 1
  fi
done
echo "Could not find KZGs in block"
exit 1
