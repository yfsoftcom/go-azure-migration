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
RUN go mod install

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -ldflags="-s -w" -o /app/bin/app /app/main.go

FROM alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
  echo http://dl-cdn.alpinelinux.org/alpine/edge/testing >> /etc/apk/repositories && \
  apk --no-cache add ca-certificates && \
  apk add tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
  echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app
COPY --from=builder /app/bin/app /app/
ENTRYPOINT [ "/app/app" ]

