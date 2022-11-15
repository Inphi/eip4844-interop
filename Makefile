devnet-up:
	docker compose --project-name eip-4844 up -d\
		execution-node\
		execution-node-2\
		prysm-beacon-node\
		prysm-beacon-node-follower\
		prysm-validator-node\
		jaeger-tracing

lodestar-up:
	docker compose --project-name eip-4844 up -d\
		execution-node\
		execution-node-2\
		lodestar-beacon-node\
		lodestar-beacon-node-follower\

devnet-down:
	docker compose --project-name eip-4844 down -v

devnet-restart: devnet-down devnet-up

devnet-clean:
	docker compose --project-name eip-4844 down --rmi local --volumes
	docker image ls 'eip-4844-interop*' --format='{{.Repository}}' | xargs -r docker rmi
	docker volume ls --filter name=eip-4844-interop --format='{{.Name}}' | xargs -r docker volume rm

.PHONY: devnet-clean
