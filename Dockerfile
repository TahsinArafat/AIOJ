# Stage 1: Build
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/aioj ./cmd/aioj

# Stage 2: Run
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S aioj && adduser -S aioj -G aioj
WORKDIR /app
COPY --from=builder /app/aioj .
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/lang ./lang
RUN chown -R aioj:aioj /app
USER aioj
EXPOSE 8080
CMD ["./aioj"]
