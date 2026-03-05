# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o location-service ./cmd/location-service

# Runtime stage — static binary only, no OS, no shell
FROM scratch
COPY --from=builder /app/location-service /location-service
EXPOSE 8080
ENTRYPOINT ["/location-service"]
