# Stage 1: Build the backend
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/aioj ./cmd/aioj && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/seed ./cmd/seed

# Stage 1b: Build the plagiarism similarity engine.
#
# This is a separate stage from the Go build on purpose: it is the only part of
# the backend that needs cargo, and keeping it isolated means the image is still
# built from a CGO_ENABLED=0 Go binary plus one static executable. The engine is
# pure compute with no C dependencies, so it links statically and can run in the
# distroless-ish runtime stage without the Rust toolchain present.
FROM rust:1.83-alpine AS rustengine
# `file` is here so the build proves the engine is statically linked rather than
# merely runnable inside the builder.
RUN apk add --no-cache musl-dev file
ENV RUSTFLAGS="-C target-feature=+crt-static"
WORKDIR /src

# Compile the dependency graph against a stub main first, so editing the engine's
# source does not invalidate the (slow) crate-compilation layer.
COPY rust/plagiarism/Cargo.toml rust/plagiarism/Cargo.lock ./
RUN mkdir -p src && echo 'fn main() {}' > src/main.rs && cargo build --release

COPY rust/plagiarism/src ./src

# `touch` is load-bearing, not cosmetic. Docker normalizes copied files' mtimes, so
# after the stub build above cargo sees sources that are no newer than its
# fingerprint and *skips the rebuild* — the image then ships `fn main() {}`, which
# exits 0 and prints nothing. The assertions after the build are the second half of
# the guard: a silent stub fails `grep`, so this cannot regress unnoticed.
RUN touch src/*.rs && \
    cargo build --release && \
    file target/release/aioj-plagiarism && \
    target/release/aioj-plagiarism --version | grep -q '^aioj-plagiarism ' && \
    printf '{"threshold":0.5,"inputs":[]}' | target/release/aioj-plagiarism | grep -q '"summary"'

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
# The repo's config.yaml is the local development profile (it allows the dev
# mail inspector and tolerates a weak JWT secret). An image is never that, so
# bake the production profile in; compose can still override it with AIOJ_ENV.
RUN sed -i 's/^environment:.*/environment: "production"/' /app/config.yaml
COPY --from=builder /app/lang ./lang
COPY --from=builder /app/internal/store/migrations ./internal/store/migrations
RUN mkdir -p /app/backups /app/testdata /app/bin
# The similarity engine. PLAGIARISM_ENGINE=auto (the default) picks it up because
# this path is pointed at explicitly; unset it and the backend uses the pure-Go
# engine instead, with no other change.
COPY --from=rustengine /src/target/release/aioj-plagiarism /app/bin/aioj-plagiarism
ENV PLAGIARISM_RUST_BIN="/app/bin/aioj-plagiarism"
RUN chown -R aioj:aioj /app
USER aioj
EXPOSE 8080
CMD ["./aioj"]
