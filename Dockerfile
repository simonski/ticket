# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Cache module downloads before copying full source
COPY go.mod go.sum ./
RUN go mod download

# Copy everything and build
COPY . .
RUN go build -o /out/tk ./cmd/ticket

# ── Stage 2: runtime ────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

# Non-root user for the runtime container
RUN adduser -D -h /home/ticket ticket
RUN mkdir -p /home/ticket/.ticket && chown ticket:ticket /home/ticket/.ticket
USER ticket
WORKDIR /home/ticket

COPY --from=builder /out/tk /usr/local/bin/tk
COPY --chmod=755 deploy/entrypoint.sh /usr/local/bin/entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/healthz || exit 1

ENTRYPOINT ["entrypoint.sh"]
CMD []
