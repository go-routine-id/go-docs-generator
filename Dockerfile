# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o docs-generator ./cmd/server

# Runtime stage — distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/docs-generator /app/docs-generator
COPY --from=builder /app/spec /app/spec

EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/docs-generator"]
CMD ["-spec", "/app/spec/index.yaml", "-port", "8080"]
