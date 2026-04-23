# syntax=docker/dockerfile:1.7

FROM golang:1.26.1 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY internal/config ./internal/config
COPY tools/admin ./tools/admin

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/create-admin ./tools/admin

FROM gcr.io/distroless/static:nonroot

WORKDIR /app

COPY --from=builder /out/create-admin /app/create-admin

ENTRYPOINT ["/app/create-admin"]
