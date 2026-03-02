# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o docs-generator cmd/server/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/docs-generator .

# Copy default spec (can be overridden with volume)
COPY --from=builder /app/api-spec.yaml .

# Expose port
EXPOSE 8080

# Run
CMD ["./docs-generator", "-spec", "./api-spec.yaml", "-port", "8080"]
