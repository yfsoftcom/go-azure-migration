# Compile stage
FROM golang:1.16.2-alpine AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY="https://goproxy.io,direct"

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -ldflags="-s -w" -o /app/bin/app /app/main.go

FROM alpine:latest AS azcopyDownload
RUN apk add wget && wget -O azcopy.tar.gz https://aka.ms/downloadazcopyprlinux && tar -xf azcopy.tar.gz

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/bin/app /app/
COPY --from=azcopyDownload azcopy/azcopy /app/
ENTRYPOINT [ "/app/app" ]

