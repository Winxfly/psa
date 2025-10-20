FROM golang:1.24.4-alpine as builder

WORKDIR /app

ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/migrator ./cmd/migrator
COPY internal/config ./internal/config

RUN go build -ldflags="-s -w" -o migrator ./cmd/migrator

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/migrator .

ENTRYPOINT ["./migrator"]