# 构建阶段
FROM golang:1.24-alpine AS builder

ARG VERSION=dev

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w -X main.version=${VERSION}" -o plugstream ./cmd/server

# 运行阶段
FROM alpine:3.21

RUN apk add --no-cache \
    ffmpeg \
    wget \
    ca-certificates

RUN addgroup -g 1000 plugstream && \
    adduser -D -u 1000 -G plugstream plugstream

WORKDIR /app

COPY --from=builder /build/plugstream .
COPY config.yaml .

RUN mkdir -p /app/cache && \
    chown -R plugstream:plugstream /app

USER plugstream

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["./plugstream"]
CMD ["-config", "config.yaml"]
