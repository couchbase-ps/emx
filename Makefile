NAME = couchbase-emx
PACKAGES = $(shell go list ./exporter/...)
GO_OS = linux
GO_ARCH = amd64
DOCKERFILE = ./Dockerfile
DOCKER_CONTEXT = .
VERSION ?= 
BUILDER_IMAGE ?=
DOCKER_REPO ?= 
DMZ_DOCKER_REPO ?=

build-linux:
	docker build --network host -t couchbase-emx --build-arg GOOS=linux --build-arg GOARCH=amd64 .

build-windows:
	docker build --network host -t couchbase-emx-win --build-arg GOOS=windows --build-arg GOARCH=amd64 .

build-macos:
	docker build --network host -t couchbase-emx-mac --build-arg GOOS=linux --build-arg GOARCH=arm64 .

