devnet-up:
	docker compose up -d\
		execution-node\
		execution-node-2\
		beacon-node\
		beacon-node-follower\
		validator-node\
		jaeger-tracing

devnet-down:
	docker compose down -v

devnet-restart: devnet-down devnet-up

devnet-clean:
	docker compose down
	docker image ls 'eip4844-interop*' --format='{{.Repository}}' | xargs -r docker rmi
	docker volume ls --filter name=eip4844-interop --format='{{.Name}}' | xargs -r docker volume rm

.PHONY: devnet-clean
