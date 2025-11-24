# Builder stage
FROM golang:1.24-alpine AS builder

WORKDIR /srv/build

# Copy vendored dependencies (generated on host with: go mod vendor)
COPY vendor ./vendor

# Copy all source files
COPY . .

# Build the application using build.sh with vendored deps
# If build.sh fails (e.g., go get fails), fall back to using vendor
RUN sh build.sh || go build -mod=vendor -o albumd.exe ./src/cli/*.go

# Runtime stage
FROM alpine:3

WORKDIR /var/lib/albumd

VOLUME /usr/share/albumd
EXPOSE 8080

# Copy only the binary and templates from builder
COPY --from=builder /srv/build/albumd.exe /usr/local/bin/albumd
COPY --from=builder /srv/build/templates /var/lib/albumd/templates

CMD ["/usr/local/bin/albumd", "-path", "/usr/share/albumd", "-thumbs", "/usr/share/albumd/thumbs"]