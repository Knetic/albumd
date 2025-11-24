#!/bin/bash

go get ./...
go build -o albumd.exe ./src/cli/*.go

if [ -f albumd.exe~ ]; then
	rm albumd.exe~
fi