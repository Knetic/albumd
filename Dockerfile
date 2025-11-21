# Builder stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files and vendor directory
COPY go.mod go.sum ./
COPY vendor ./vendor

# Copy source code
COPY Server.go templater.go thumbnailer.go ./
COPY src ./src
COPY templates ./templates

# Build the application for Linux using vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o albumd ./src/cli/*.go

# Runtime stage
FROM alpine:3

WORKDIR /var/lib/albumd

VOLUME /usr/share/albumd
EXPOSE 8080

# Copy only the binary and templates from builder
COPY --from=builder /build/albumd /usr/local/bin/albumd
COPY --from=builder /build/templates /var/lib/albumd/templates

CMD ["/usr/local/bin/albumd", "-path", "/usr/share/albumd", "-thumbs", "/usr/share/albumd/thumbs"]