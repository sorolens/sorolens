integration-test:
	docker-compose up -d --build
	@echo "Waiting for API to be ready..."
	@until curl -s http://localhost:8080/readyz > /dev/null; do sleep 1; done
	go mod tidy
	go test ./... -v -tags=integration
	docker-compose down
