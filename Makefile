all: containerized_build

build: clean fmt
	@mkdir -p .bin

	@go get ./...
	@go build -o .bin/albumd ./src/cli/*.go

clean:
	@rm -rf .bin

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