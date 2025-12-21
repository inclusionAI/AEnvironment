# Dockerfile for Docker Controller
FROM golang:1.21-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build the docker controller
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o docker-controller ./cmd/main.go

# Use a minimal base image
FROM alpine:latest

RUN apk --no-cache add ca-certificates docker-cli

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /workspace/docker-controller .

# Mount Docker socket (should be mounted at runtime)
# VOLUME ["/var/run/docker.sock"]

EXPOSE 8080

ENTRYPOINT ["./docker-controller"]

