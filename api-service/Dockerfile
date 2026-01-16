# Build stage
FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /workspace

# Configure Go environment
RUN go env -w GO111MODULE=on && \
    go env -w GOPROXY=https://goproxy.cn,direct && \
    go env -w GOSUMDB=sum.golang.google.cn && \
    go env -w GOPRIVATE=""

# Copy go.work first (for workspace support)
COPY go.work ./

# Copy go.mod and go.sum files first for better caching
COPY api-service/go.mod api-service/go.sum ./api-service/
COPY envhub/go.mod envhub/go.sum ./envhub/
COPY controller/go.mod controller/go.sum ./controller/

# Download dependencies (this layer will be cached if go.mod/go.sum don't change)
WORKDIR /workspace/api-service
RUN set -e; \
    if ! go mod download -x; then \
      echo "Download failed, retrying..."; \
      sleep 5; \
      go mod download; \
    fi

WORKDIR /workspace

# Copy source code after dependencies are downloaded
COPY envhub ./envhub
COPY controller ./controller
COPY api-service ./api-service

# Build
WORKDIR /workspace/api-service
RUN go build -o api-service .

WORKDIR /workspace

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /workspace/api-service/api-service /usr/bin/api-service

EXPOSE 8080

ENTRYPOINT ["/usr/bin/api-service"]
