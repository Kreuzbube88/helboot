# HELBOOT — single image, single container (ADR-0009).
# Build from the repository root:
#   docker build -f deploy/docker/Dockerfile -t helboot .

# Stage 1: build the frontend.
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: build the Go binary with the frontend embedded.
FROM golang:1.24-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Replace the placeholder with the real UI before embedding (go:embed).
RUN rm -rf internal/web/dist
COPY --from=frontend /src/frontend/dist internal/web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /helboot ./cmd/helboot

# Stage 3: fetch the iPXE bootstrap binaries served via TFTP (ADR-0011).
FROM alpine:3.24 AS ipxe
RUN apk add --no-cache curl
COPY deploy/scripts/fetch-ipxe.sh /fetch-ipxe.sh
RUN sh /fetch-ipxe.sh /ipxe

# Stage 4: minimal runtime image.
FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S helboot && adduser -S -G helboot helboot
COPY --from=backend /helboot /usr/local/bin/helboot
COPY providers/ /app/providers/
# Seed iPXE binaries; the entrypoint copies them into the data volume's
# assets/tftp directory on first start if missing.
COPY --from=ipxe /ipxe/ /app/seed/tftp/
COPY deploy/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV HELBOOT_DATA_DIR=/data \
    HELBOOT_PROVIDERS_DIR=/app/providers \
    HELBOOT_HTTP_ADDR=:8080 \
    HELBOOT_LOG_FORMAT=json

VOLUME ["/data"]
# TCP 8080: web UI/API/HTTP boot. UDP 67/4011: DHCP & ProxyDHCP.
# UDP 69: TFTP. DHCP/TFTP need host networking in practice (ADR-0006);
# the EXPOSE lines are documentation for bridge-only UI evaluation.
EXPOSE 8080/tcp 67/udp 69/udp 4011/udp

# Note: DHCP/TFTP bind privileged ports, so the container currently
# starts as root; the network services will drop privileges after
# binding once they land. Until then the HTTP-only server can also run
# with `--user helboot`.
ENTRYPOINT ["/entrypoint.sh"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/v1/health || exit 1
