# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.26.4-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS builder

WORKDIR /src

# Cache module downloads before copying full source
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy everything and build
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux GOARCH=amd64 go build -o /out/tk ./cmd/tk

FROM scratch AS artifact

COPY --from=builder /out/tk /tk-linux

# ── Stage 2: runtime ────────────────────────────────────────────────────────
FROM alpine:3.21@sha256:c3f8e73fdb79deaebaa2037150150191b9dcbfba68b4a46d70103204c53f4709

RUN apk add --no-cache ca-certificates

# Non-root user for the runtime container
RUN adduser -D -h /home/ticket ticket
RUN mkdir -p /data /home/ticket && chown -R ticket:ticket /data /home/ticket
ENV TICKET_HOME=/data
ENV TICKET_DATA_DIR=/data
ENV TICKET_DB_PATH=/data/ticket.db
ENV TICKET_SERVER_ADDR=0.0.0.0:8080
WORKDIR /home/ticket

COPY --from=builder /out/tk /usr/local/bin/tk
COPY --chmod=755 deploy/entrypoint.sh /usr/local/bin/entrypoint.sh

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/healthz || exit 1

ENTRYPOINT ["entrypoint.sh"]
CMD []
