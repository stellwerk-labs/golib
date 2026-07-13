test:
	docker compose up -d --wait db
	go test ./... -v -cover -race
	docker compose down -v --remove-orphans
