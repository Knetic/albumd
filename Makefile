all: container

build: clean fmt
	@mkdir -p .bin

	@go get ./...
	@go build -o .bin/albumd ./src/cli/*.go

clean:
	@rm -rf .bin

fmt:
	@go fmt .

container:
	@docker build -t albumd .

containerized_build:
	@docker build -t albumd .