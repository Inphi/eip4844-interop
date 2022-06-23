devnet-up:
	docker-compose up -d\
		execution-node\
		execution-node-2\
		beacon-node\
		beacon-node-follower\
		validator-node\
		jaeger-tracing

devnet-clean:
	docker-compose down
	docker image ls 'eip4844-interop*' --format='{{.Repository}}' | xargs docker rmi
	docker volume ls --filter name=eip4844-interop --format='{{.Name}}' | xargs docker volume rm

.PHONE: devnet-clean
