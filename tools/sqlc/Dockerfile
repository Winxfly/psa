FROM golang:1.24.4-alpine

RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0

WORKDIR /app

COPY sqlc.yaml .

ENTRYPOINT ["sqlc"]