# syntax=docker/dockerfile:1
# Rootless Podman / Docker – alle Assets sind go:embed eingebettet, nur Binary nötig

# --- Build Stage ---
FROM docker.io/library/golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# cmd/homeport ist der Entry Point; CGO_ENABLED=0 für statisches Binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/homeport ./cmd/homeport

# --- Final Stage ---
FROM docker.io/library/alpine:3.21

# Non-root user
RUN addgroup -S homeport && adduser -S -G homeport -u 1000 homeport

WORKDIR /app

COPY --from=builder /out/homeport /app/homeport

# Persistent data directory
RUN mkdir -p /app/data && chown homeport:homeport /app/data

EXPOSE 8855
VOLUME /app/data

USER homeport

ENTRYPOINT ["/app/homeport"]
