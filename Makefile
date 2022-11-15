devnet-up:
	docker compose --project-name eip4844-interop up -d\
		execution-node\
		execution-node-2\
		prysm-beacon-node\
		prysm-beacon-node-follower\
		prysm-validator-node\
		jaeger-tracing

lodestar-up:
	docker compose --project-name eip4844-interop up -d\
		execution-node\
		execution-node-2\
		lodestar-beacon-node\
		lodestar-beacon-node-follower\

devnet-down:
	docker compose down -v

devnet-restart: devnet-down devnet-up

devnet-clean:
	docker compose down
	docker compose down --volumes
	docker image ls 'eip4844-interop*' --format='{{.Repository}}' | xargs -r docker image rm -f
	docker volume ls --filter name=eip4844-interop --format='{{.Name}}' | xargs -r docker volume rm -f

.PHONY: devnet-clean
