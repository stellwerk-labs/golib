test:
	docker compose up -d --wait vault
	go test ./... -v -cover -race
	docker compose down -v --remove-orphans
