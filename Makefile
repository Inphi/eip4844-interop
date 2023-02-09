SERVICES=geth-1\
	 geth-2\
	 besu-1 \
	 prysm-beacon-node\
	 prysm-beacon-node-follower\
	 prysm-validator-node\
	 lighthouse-beacon-node\
	 lighthouse-beacon-node-follower\
	 lighthouse-validator-node\
	 jaeger-tracing

devnet-setup: devnet-clean
	docker compose --project-name eip4844-interop up genesis-generator

devnet-build:
	docker compose --project-name eip4844-interop build ${SERVICES}

# First build then setup so we don't start after the genesis_delay
devnet-up: devnet-build devnet-setup
	docker compose --project-name eip4844-interop up -d ${SERVICES}

lighthouse-up: devnet-build devnet-setup
	docker compose --project-name eip4844-interop up -d --build\
		geth-1\
		geth-2\
		lighthouse-beacon-node\
		lighthouse-beacon-node-follower\
		lighthouse-validator-node

lodestar-up:
	docker compose --project-name eip4844-interop up -d\
		geth-1\
		geth-2\
		lodestar-beacon-node\
		lodestar-beacon-node-follower\

lighthouse-prysm: devnet-setup
	docker compose --project-name eip4844-interop up -d --build lighthouse-validator-node
	sleep 300
	docker compose --project-name eip4844-interop up -d --build prysm-beacon-node-follower

besu-prysm-up: devnet-build devnet-setup
	docker compose --project-name eip4844-interop up -d --build \
		besu-1 \
		prysm-beacon-node-besu-el \
		prysm-beacon-node-follower \
		prysm-validator-node-besu-el


devnet-down:
	docker compose --project-name eip4844-interop down -v

devnet-restart: devnet-down devnet-up

devnet-clean:
	docker compose --project-name eip4844-interop down --rmi local --volumes
	docker image ls 'eip4844-interop*' --format='{{.Repository}}' | xargs -r docker rmi
	docker volume ls --filter name=eip4844-interop --format='{{.Name}}' | xargs -r docker volume rm

blobtx-test: devnet-setup
	go run ./tests/blobtx $(el)

pre4844-test: devnet-setup
	go run ./tests/pre-4844 $(el)

initial-sync-test: devnet-setup
	go run ./tests/initial-sync $(el)

fee-market-test: devnet-setup
	go run ./tests/fee-market $(el)

.PHONY: devnet-clean
