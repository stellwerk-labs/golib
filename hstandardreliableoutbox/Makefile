test:
	docker compose up -d --wait
	go test ./... -v -cover -race
	docker compose down -v --remove-orphans -t 1
