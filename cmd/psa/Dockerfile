# syntax=docker/dockerfile:1.7

FROM golang:1.26.1 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/psa ./cmd/psa

FROM gcr.io/distroless/static:nonroot

WORKDIR /app

COPY --from=builder /out/psa /app/psa

ENTRYPOINT ["/app/psa"]
