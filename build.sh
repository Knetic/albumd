#!/bin/bash

# Get version tag if provided, otherwise use 'latest'
TAG="${1:-latest}"

# Ensure vendor directory exists for Docker build
echo "Vendoring dependencies..."
go mod vendor

# Build the Docker image using the multi-stage Dockerfile
echo "Building Docker image albumd:${TAG}..."
docker build -t albumd:${TAG} .
