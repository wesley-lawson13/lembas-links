-include .env
export

.PHONY: run stop build test seed seed-dev migrate logs generate

run:
	docker compose up --build

stop:
	docker compose down

build:
	docker compose build

test:
	cd api && go test ./models/... -v

migrate:
	docker compose exec api migrate -path /db/migrations -database ${DATABASE_URL} up

seed:
	docker compose exec postgres psql -U ${POSTGRES_USER} -d ${POSTGRES_DB} -f /db/seeds/quotes.sql

seed-dev:
	docker compose exec postgres psql -U ${POSTGRES_USER} -d ${POSTGRES_DB} -c "INSERT INTO api_keys (key, name) VALUES ('test-api-key-123', 'dev key') ON CONFLICT DO NOTHING;"

logs:
	docker compose logs -f

# Must be called with the virtual environment activated
generate:
	cd nlp-service && python generate_slugs.py
