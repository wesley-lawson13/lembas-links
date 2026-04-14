FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .
RUN go build -o lembas-links .

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /build/lembas-links .
COPY db/migrations/ /db/migrations/
COPY db/seeds/ /db/seeds/

EXPOSE 8080
CMD ["./lembas-links"]
