# syntax=docker/dockerfile:1

# --- Build Stage ---
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Copy go.mod and go.sum files
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the static binary
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /out/homeport .

# --- Final Stage ---
FROM alpine:3.21

# Create a non-root user
RUN addgroup -S homeport && adduser -S -G homeport -u 1000 homeport

# Set the working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /out/homeport /app/homeport

# Copy static assets and templates
COPY static ./static
COPY templates ./templates

# Create a directory for the database and set permissions
RUN mkdir -p /app/data && chown -R homeport:homeport /app/data

# Expose the application port
EXPOSE 8854

# Define the volume for persistent data
VOLUME /app/data

# Switch to the non-root user
USER homeport

# Run the application
ENTRYPOINT ["/app/homeport"]
