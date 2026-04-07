include .env
export

.PHONY: run stop build test seed migrate

run:
	docker compose up --build

stop:
	docker compose down

build:
	docker compose build

test:
	cd api && go test ./...

migrate:
	docker compose exec api migrate -path /db/migrations -database ${DATABASE_URL} up

seed:
	docker compose exec postgres psql -U ${POSTGRES_USER} -d ${POSTGRES_DB} -f /db/seeds/quotes.sql

logs:
	docker compose logs -f

# Must be called with the virtual environment activated
generate:
	cd nlp-service && python generate_slugs.py
