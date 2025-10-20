FROM golang:1.24.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY internal/config ./internal/config
COPY tools/admin/main.go .

RUN go build -o create-admin main.go

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/create-admin .

ENTRYPOINT ["./create-admin"]