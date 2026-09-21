.PHONY: build run test test-ci vet fmt lint mocks migration-up migration-down migration-force docker-up docker-down clean

build:
	go build -o grpc ./cmd/grpc

run:
	go run ./cmd/grpc

test:
	go test ./...

test-ci:
	go test -race -cover ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run --timeout=5m

mocks:
	./scripts/generate_mocks.sh

migration-up:
	migrate -path migrations -database "$$POSTGRES_DSN" up

migration-down:
	migrate -path migrations -database "$$POSTGRES_DSN" down

migration-force:
	migrate -path migrations -database "$$POSTGRES_DSN" force $(VERSION)

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

clean:
	rm -f grpc coverage.out
