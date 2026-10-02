FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /main .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /main /main

# Run as an unprivileged user (nobody); scratch has no passwd file, so use the numeric id.
USER 65534:65534

# Quiet, production-mode Gin (no per-route debug output).
ENV GIN_MODE=release

EXPOSE 8080

# scratch has no shell or curl, so the binary probes itself.
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=5 CMD ["/main", "-healthcheck"]

CMD ["/main"]
