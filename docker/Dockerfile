# Scout — full image (CLI + Chromium)
# Usage: docker build -t scout:latest .
# Run:   docker run --rm --shm-size=2g scout:latest version

# Stage 1: Build the scout binary
FROM golang:1.25 AS builder

WORKDIR /src
COPY ../go.mod go.sum ./
RUN go mod download
COPY .. .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /scout ./cmd/scout/

# Stage 2: Runtime with Chromium
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    fonts-noto \
    fonts-noto-cjk \
    fonts-noto-color-emoji \
    dumb-init \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Non-root user
RUN groupadd -r scout && useradd -r -g scout -m -d /home/scout scout

COPY --from=builder /scout /usr/local/bin/scout

ENV SCOUT_HEADLESS=true \
    SCOUT_NO_SANDBOX=true \
    CHROME_BIN=/usr/bin/chromium

# gRPC (9551) and pairing (9552)
EXPOSE 9551 9552

# /dev/shm must be mounted with sufficient size at runtime:
#   docker run --shm-size=2g ...
# or via tmpfs in docker-compose

USER scout
WORKDIR /home/scout

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD ["scout", "version"]

ENTRYPOINT ["dumb-init", "--", "scout"]
