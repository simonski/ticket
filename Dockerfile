# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Cache module downloads before copying full source
COPY go.mod go.sum ./
RUN go mod download

# Copy everything and build
COPY . .
RUN go build -o /out/ticket ./cmd/ticket

# ── Stage 2: runtime ────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

# Non-root user for the runtime container
RUN adduser -D -h /home/ticket ticket
USER ticket
WORKDIR /home/ticket

COPY --from=builder /out/ticket /usr/local/bin/ticket

EXPOSE 8080

ENTRYPOINT ["ticket"]
CMD ["server"]
