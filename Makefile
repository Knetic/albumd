all: container

build:
	@./build.sh

clean:
	@rm -rf .bin

fmt:
	@go fmt .

container:
	@docker build -t albumd .