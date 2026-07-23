-include .env
export

.PHONY: run stop build test test-rate e2e seed seed-dev migrate logs generate

run:
	docker compose up --build

stop:
	docker compose down

build:
	docker compose build

test:
	cd api && go test ./models/... -v

test-rate:
	cd api && go test ./middleware/... -v

# End-to-end smoke test over real HTTP — assumes the stack is up (make run)
e2e:
	./scripts/e2e_smoke_test.sh

migrate:
	docker compose exec api migrate -path /db/migrations -database ${DATABASE_URL} up

seed:
	docker compose exec postgres psql -U ${POSTGRES_USER} -d ${POSTGRES_DB} -f /db/seeds/quotes.sql

# Inserts the SHA-256 hash of 'test-api-key-123' — pass that plaintext value
# as the Bearer token when testing locally. POST /session is now the primary
# key-provisioning path; this is a manual-testing convenience only.
seed-dev:
	docker compose exec postgres psql -U ${POSTGRES_USER} -d ${POSTGRES_DB} -c "INSERT INTO api_keys (key, name, expires_at) VALUES ('a2e4ab0472c808a1ff2ce147ae4f6cd9ecd8bcc8a49c48350f97e6811ace7464', 'dev key', NOW() + INTERVAL '30 days') ON CONFLICT DO NOTHING;"

logs:
	docker compose logs -f

# Must be called with the virtual environment activated
generate:
	cd nlp-service && python generate_slugs.py
