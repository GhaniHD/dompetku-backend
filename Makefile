TEST_DB_HOST ?= localhost
TEST_DB_PORT ?= 5433
TEST_DB_USER ?= postgres
TEST_DB_NAME ?= dompetku_ci_test
export TEST_DB_HOST TEST_DB_PORT TEST_DB_USER TEST_DB_PASSWORD TEST_DB_NAME

.PHONY: run build vet test

run:
	go run ./cmd/app

build:
	go build ./...

vet:
	go vet ./...

test:
	go test -race -count=1 ./...
