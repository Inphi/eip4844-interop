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
		lodestar-beacon-node-follower\

devnet-down:
	docker compose down -v

devnet-restart: devnet-down devnet-up

devnet-clean:
	docker compose down --rmi local --volumes
	docker image ls '`basename $PWD`-interop*' --format='{{.Repository}}' | xargs -r docker rmi
	docker volume ls --filter name=`basename $PWD`-interop --format='{{.Name}}' | xargs -r docker volume rm

.PHONY: devnet-clean
