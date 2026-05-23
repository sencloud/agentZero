# syntax=docker/dockerfile:1.6

FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/agentzero ./cmd/agentzero

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S agentzero && adduser -S agentzero -G agentzero && \
    mkdir -p /var/lib/agentzero && chown agentzero:agentzero /var/lib/agentzero
ENV TZ=Asia/Shanghai
ENV PORT=8080
ENV DB_PATH=/var/lib/agentzero/agentzero.db
WORKDIR /app
COPY --from=build /out/agentzero /app/agentzero
USER agentzero
EXPOSE 8080
VOLUME ["/var/lib/agentzero"]
HEALTHCHECK --interval=20s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/agentzero"]
