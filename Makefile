devnet-up:
	docker-compose up -d execution-node beacon-node validator-node

devnet-clean:
	docker image ls 'eip4844-interop*' --format='{{.Repository}}' | xargs docker rmi
	docker volume ls --filter name=eip4844-interop --format='{{.Name}}' | xargs docker volume rm

.PHONE: devnet-clean
