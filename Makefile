all: containerized_build

export GOPATH=$(CURDIR)/
export GOBIN=$(CURDIR)/.temp/bin
export GOCACHE=$(CURDIR)/.temp/cache

build: clean fmt
	@mkdir -p .bin
	@mkdir -p .temp/bin
	@mkdir -p .temp/cache

	@go get ./...
	@go build -o .bin/albumd .

clean:
	@rm -rf .bin
	@rm -rf .temp

fmt:
	@go fmt .

container: build
	@docker build -t albumd .

containerized_build:

	@docker run \
		--rm \
		-v "$(CURDIR)":"/srv/build":rw \
		-u "$(shell id -u $(whoami)):$(shell id -g $(whoami))" \
		golang:1.24 \
		bash -c \
		"cd /srv/build; make build"