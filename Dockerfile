# ============================================================
# Stage 1: Build all Go binaries
# ============================================================
FROM golang:1.26-alpine AS builder

ARG HTTP_PROXY
ARG HTTPS_PROXY

ENV GOPROXY=https://goproxy.cn,direct

# Use Aliyun Alpine mirror
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories

RUN apk add --no-cache git

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/api       ./cmd/api
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/worker    ./cmd/worker
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/outbox-relay ./cmd/outbox-relay
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/consumer  ./cmd/consumer

# ============================================================
# Stage 2: API runtime
# ============================================================
FROM alpine:3.21 AS api

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates-bundle ffmpeg
COPY --from=builder /bin/api /usr/local/bin/api

EXPOSE 8080
ENTRYPOINT ["api"]

# ============================================================
# Stage 3: Worker runtime
# ============================================================
FROM alpine:3.21 AS worker

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates-bundle ffmpeg
COPY --from=builder /bin/worker /usr/local/bin/worker

ENTRYPOINT ["worker"]

# ============================================================
# Stage 4: Outbox Relay runtime
# ============================================================
FROM alpine:3.21 AS outbox-relay

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates-bundle
COPY --from=builder /bin/outbox-relay /usr/local/bin/outbox-relay

EXPOSE 9090
ENTRYPOINT ["outbox-relay"]

# ============================================================
# Stage 5: Kafka Consumer runtime
# ============================================================
FROM alpine:3.21 AS consumer

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates-bundle
COPY --from=builder /bin/consumer /usr/local/bin/consumer

EXPOSE 9090
ENTRYPOINT ["consumer"]

# ============================================================
# Stage 6: Build migration tool
# ============================================================
FROM golang:1.26-alpine AS goose-builder

ENV GOPROXY=https://goproxy.cn,direct

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories
RUN apk add --no-cache git
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# ============================================================
# Stage 7: Database migration runtime
# ============================================================
FROM alpine:3.21 AS migrate

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates-bundle
COPY --from=goose-builder /go/bin/goose /usr/local/bin/goose
COPY migrations /migrations

ENTRYPOINT ["goose", "-dir", "/migrations"]
