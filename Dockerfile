# Stage 1: Build
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/aioj ./cmd/aioj && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/seed ./cmd/seed

# Stage 2: Run
FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata \
    g++ gcc make musl-dev python3 openjdk21-jdk rust cargo nodejs npm bash tar
RUN apk add --no-cache --repository=https://dl-cdn.alpinelinux.org/alpine/edge/main postgresql18-client
RUN addgroup -S aioj && adduser -S aioj -G aioj
WORKDIR /app
COPY --from=builder /app/aioj .
COPY --from=builder /app/seed .
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/lang ./lang
COPY --from=builder /app/internal/store/migrations ./internal/store/migrations
RUN mkdir -p /app/backups /app/testdata
RUN chown -R aioj:aioj /app
USER aioj
EXPOSE 8080
CMD ["./aioj"]
