devnet-up:
	docker compose up -d\
		execution-node\
		execution-node-2\
		prysm-beacon-node\
		prysm-beacon-node-follower\
		prysm-validator-node\
		jaeger-tracing

lodestar-up:
	docker compose up -d\
		execution-node\
		execution-node-2\
		lodestar-beacon-node\
		lodestar-beacon-follower\

devnet-down:
	docker compose down -v

devnet-restart: devnet-down devnet-up

devnet-clean:
	docker compose down
	docker image ls 'interop*' --format='{{.Repository}}' | xargs -r docker rmi
	docker volume ls --filter name=interop --format='{{.Name}}' | xargs -r docker volume rm

.PHONY: devnet-clean
