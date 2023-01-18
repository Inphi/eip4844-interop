SERVICES=execution-node\
	 execution-node-2\
	 prysm-beacon-node\
	 prysm-beacon-node-follower\
	 prysm-validator-node\
	 jaeger-tracing

devnet-setup: devnet-clean
	docker compose --project-name eip4844-interop up genesis-generator

devnet-build:
	docker compose --project-name eip4844-interop build ${SERVICES}

# First build then setup so we don't start after the genesis_delay
devnet-up: devnet-build devnet-setup
	docker compose --project-name eip4844-interop up -d ${SERVICES}

lighthouse-up:
	touch ./lighthouse/generated-genesis.json
	touch ./lighthouse/generated-config.yaml
	docker compose --project-name eip4844-interop up -d --build\
		execution-node\
		execution-node-2\
		lighthouse-beacon-node\
		lighthouse-beacon-node-follower\
		lighthouse-validator-node

lodestar-up:
	docker compose --project-name eip4844-interop up -d\
		execution-node\
		execution-node-2\
		lodestar-beacon-node\
		lodestar-beacon-node-follower\

devnet-down:
	docker compose --project-name eip4844-interop down -v

devnet-restart: devnet-down devnet-up

devnet-clean:
	docker compose --project-name eip4844-interop down --rmi local --volumes
	docker image ls 'eip4844-interop*' --format='{{.Repository}}' | xargs -r docker rmi
	docker volume ls --filter name=eip4844-interop --format='{{.Name}}' | xargs -r docker volume rm

prysm-blobtx-test: devnet-clean devnet-setup
	go run ./tests/blobtx prysm

.PHONY: devnet-clean
