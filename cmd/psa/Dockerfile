FROM golang:1.24.4 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o psa ./cmd/psa

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /app/psa /psa

ENTRYPOINT ["/psa"]