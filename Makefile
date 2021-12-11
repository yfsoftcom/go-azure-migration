PROJECTNAME=$(shell basename "$(PWD)")
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin

all: build

install:
	go mod download

build: install
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(GOBIN)/app ./main.go

docker:
	docker build --tag azure-migration:go --tag yfsoftcom/azure-migration:latest .