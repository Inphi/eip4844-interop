devnet-up:
	docker compose --project-name eip4844-interop up -d\
		execution-node\
		execution-node-2\
		prysm-beacon-node\
		prysm-beacon-node-follower\
		prysm-validator-node\
		jaeger-tracing

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

nethermind-up:
	docker compose --project-name eip4844-interop up -d\
		execution-node\
		prysm-beacon-node\
		prysm-validator-node\
		nethermind-execution-node\
		nethermind-execution-node-2\
		prysm-beacon-node-follower-2\
		lodestar-beacon-node-follower-2

devnet-down:
	docker compose --project-name eip4844-interop down -v

devnet-restart: devnet-down devnet-up

devnet-clean:
	docker compose --project-name eip4844-interop down --rmi local --volumes
	docker image ls 'eip4844-interop*' --format='{{.Repository}}' | xargs -r docker rmi
	docker volume ls --filter name=eip4844-interop --format='{{.Name}}' | xargs -r docker volume rm

.PHONY: devnet-clean
