#!/bin/bash

go get ./...
go build -o albumd.exe ./src/cli/*.go

rm ./albumd.exe~