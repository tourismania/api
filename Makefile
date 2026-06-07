DB_URL ?= ${DATABASE_SERVER}://${DATABASE_USER}:${DATABASE_PASSWORD}@${DATABASE_HOST}:${DATABASE_PORT}/${DATABASE_NAME}?sslmode=${DATABASE_SSLMODE}

include .env

info:
	echo "Makefile for api"

migrate-up: ## Apply all migrations
	migrate -path ./migrations/${DATABASE_SERVER} -database "$(DB_URL)" up

migrate-down: ## Roll back last migration
	migrate -path ./migrations/${DATABASE_SERVER} -database "$(DB_URL)" down 1

migrate-new: ## Create new migration: make migrate-new name=foo
	migrate create -ext sql -dir ./migrations/${DATABASE_SERVER} -seq $(name)

sqlc: ## Regenerate sqlc code
	sqlc generate

swag: ## Generate OpenAPI docs into ./docs
	swag init -g cmd/server/main.go -o docs/swagger

jwt-keys: ## Generate RSA keypair for JWT (RS256)
	openssl genpkey -algorithm RSA -out config/jwt/private.pem -pkeyopt rsa_keygen_bits:2048
	openssl rsa -pubout -in config/jwt/private.pem -out config/jwt/public.pem
	chmod 644 config/jwt/private.pem config/jwt/public.pem # TODO: нужно подготовить инфру по работе с secrets файлами и убрать это
