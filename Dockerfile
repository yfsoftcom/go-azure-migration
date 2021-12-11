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
RUN apk add wget && wget https://azcopyvnext.azureedge.net/release20211027/azcopy_linux_amd64_10.13.0.tar.gz && tar -xvf azcopy_li* && mv azcopy_li*/azcopy .

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/bin/app /app/
COPY --from=azcopyDownload /app/azcopy /app/
ENTRYPOINT [ "/app/app" ]

