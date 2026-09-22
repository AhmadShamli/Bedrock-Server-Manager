# Stage 1: Build React Frontend Assets
FROM node:22-bookworm-slim AS web-builder

WORKDIR /web

COPY web/package.json web/package-lock.json* ./
RUN npm ci || npm install

COPY web/ ./
RUN npm run build

# Stage 2: Compile Static Go Binary with Embedded Frontend
FROM golang:1.26-bookworm AS go-builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy production dist into web/dist so //go:embed embeds it
COPY --from=web-builder /web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /build/bedrock-server-manager ./cmd/manager

# Stage 3: Minimal Production Runtime Image
FROM debian:bookworm-slim AS runtime

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    iptables \
    iproute2 \
    ufw \
    util-linux \
    tini \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /build/bedrock-server-manager /usr/local/bin/bedrock-server-manager
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh /usr/local/bin/bedrock-server-manager

VOLUME ["/data"]

EXPOSE 8080

ENTRYPOINT ["/usr/bin/tini", "-s", "--", "/app/docker-entrypoint.sh"]
CMD ["/usr/local/bin/bedrock-server-manager"]
