.PHONY: dev
dev:
	docker-compose up 

.PHONY: down
down:
	docker-compose down

.PHONY: test
test:
	docker-compose run --rm app go test ./...

.PHONY: shell
shell:
	docker-compose run --rm app sh